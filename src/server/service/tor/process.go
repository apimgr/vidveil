// SPDX-License-Identifier: MIT
package tor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cretz/bine/control"
)

var controlPortPattern = regexp.MustCompile(`^PORT=(127\.0\.0\.1:[0-9]{1,5})\n$`)
var onionAddressPattern = regexp.MustCompile(`^[a-z2-7]{56}\.onion$`)

type torProcess struct {
	cmd         *exec.Cmd
	done        chan struct{}
	output      *processOutput
	err         error
	ctrl        *control.Conn
	eventCtrl   *control.Conn
	eventCancel context.CancelFunc
}

type processOutput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *processOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	if n >= 32*1024 {
		o.buf.Reset()
		p = p[n-32*1024:]
	} else if o.buf.Len()+n > 32*1024 {
		old := o.buf.Bytes()
		keep := 32*1024 - n
		copy(old, old[len(old)-keep:])
		o.buf.Reset()
		_, _ = o.buf.Write(old[:keep])
	}
	_, err := o.buf.Write(p)
	return n, err
}

func (o *processOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func spawnTor(ctx context.Context, binary, torrc, dataDir string) (*torProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Remove(filepath.Join(dataDir, "control_port")); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale control port file: %w", err)
	}
	cmd := exec.Command(binary, "-f", torrc)
	output := &processOutput{}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &torProcess{cmd: cmd, done: make(chan struct{}), output: output}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

func waitControlPort(ctx context.Context, p *torProcess, file string, timeout time.Duration) (string, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-p.done:
			return "", fmt.Errorf("tor exited before opening control port: %v: %s", p.err, p.output.String())
		default:
		}
		if data, err := os.ReadFile(file); err == nil {
			if match := controlPortPattern.FindSubmatch(data); match != nil {
				addr := string(match[1])
				host, portText, splitErr := net.SplitHostPort(addr)
				port, portErr := strconv.Atoi(portText)
				if splitErr == nil && host == "127.0.0.1" && portErr == nil && port > 0 && port <= 65535 {
					return addr, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("control_port file not ready after %s", timeout)
		case <-tick.C:
		}
	}
}

func dialControl(ctx context.Context, address string) (*control.Conn, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	ctrl := control.NewConn(textproto.NewConn(conn))
	if err := ctrl.Authenticate(""); err != nil {
		_ = ctrl.Close()
		return nil, err
	}
	return ctrl, nil
}

func startTorProcess(ctx context.Context, binary, torrc, dataDir string, startupTimeout time.Duration) (*torProcess, *control.Conn, *control.Conn, error) {
	for attempt := 0; attempt < 2; attempt++ {
		p, err := spawnTor(ctx, binary, torrc, dataDir)
		if err != nil {
			return nil, nil, nil, err
		}
		startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
		address, err := waitControlPort(startupCtx, p, filepath.Join(dataDir, "control_port"), startupTimeout)
		cancel()
		if err != nil {
			_ = stopTor(p)
			return nil, nil, nil, err
		}
		ctrl, err := dialControl(ctx, address)
		if err == nil {
			p.ctrl = ctrl
			var eventCtrl *control.Conn
			eventCtrl, err = dialControl(ctx, address)
			if err == nil {
				p.ctrl = ctrl
				p.eventCtrl = eventCtrl
				return p, ctrl, eventCtrl, nil
			}
			_ = ctrl.Close()
			p.ctrl = nil
		}
		_ = stopTor(p)
		_ = os.Remove(filepath.Join(dataDir, "control_auth_cookie"))
		if attempt == 1 {
			return nil, nil, nil, fmt.Errorf("tor control authentication failed after one restart: %w: %s", err, p.output.String())
		}
	}
	return nil, nil, nil, fmt.Errorf("tor control authentication failed")
}

func bootstrapTor(ctrl, eventCtrl *control.Conn) (<-chan control.Event, context.CancelFunc, error) {
	events := make(chan control.Event, 64)
	if err := eventCtrl.AddEventListener(events, control.EventCodeStatusClient, control.EventCodeHSDesc); err != nil {
		return nil, nil, err
	}
	eventCtx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = eventCtrl.HandleEvents(eventCtx)
	}()
	if err := ctrl.SetConf(&control.KeyVal{Key: "DisableNetwork", Val: "0"}); err != nil {
		cancel()
		return nil, nil, err
	}
	return events, cancel, nil
}

func waitBootstrapProgress(ctx context.Context, ctrl *control.Conn, events <-chan control.Event, onWarning func(string), onProgress func(int)) error {
	if values, err := ctrl.GetInfo("status/bootstrap-phase"); err == nil {
		for _, value := range values {
			if progress, ok := parseBootstrapProgress(value.Val); ok && onProgress != nil {
				onProgress(progress)
			}
			if strings.Contains(value.Val, "PROGRESS=100") {
				return nil
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-events:
			status, ok := event.(*control.StatusEvent)
			if !ok || status.Type != control.EventCodeStatusClient {
				continue
			}
			if (status.Severity == "WARN" || status.Severity == "ERR") && status.Arguments["REASON"] != "" && onWarning != nil {
				onWarning(status.Arguments["REASON"])
			}
			if status.Action != "BOOTSTRAP" {
				continue
			}
			if progress, ok := parseBootstrapProgress(status.Arguments["PROGRESS"]); ok && onProgress != nil {
				onProgress(progress)
			}
			if status.Arguments["PROGRESS"] == "100" {
				return nil
			}
		}
	}
}

func parseBootstrapProgress(value string) (int, bool) {
	progress, err := strconv.Atoi(value)
	return progress, err == nil && progress >= 0 && progress <= 100
}

func waitDescriptorUpload(ctx context.Context, events <-chan control.Event, onionAddress string, onFailure func(string)) error {
	targetAddress := strings.TrimSuffix(onionAddress, ".onion")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-events:
			descriptor, ok := event.(*control.HSDescEvent)
			if !ok || (descriptor.Address != "" && descriptor.Address != targetAddress) {
				continue
			}
			switch descriptor.Action {
			case "UPLOADED":
				return nil
			case "FAILED":
				if onFailure != nil {
					onFailure(descriptor.Reason)
				}
			}
		}
	}
}

func waitHostname(ctx context.Context, filename string) (string, error) {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		if data, err := os.ReadFile(filename); err == nil {
			address := strings.TrimSpace(string(data))
			if onionAddressPattern.MatchString(address) {
				return address, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
		}
	}
}

func socksListener(ctrl *control.Conn) (string, error) {
	values, err := ctrl.GetInfo("net/listeners/socks")
	if err != nil {
		return "", err
	}
	for _, value := range values {
		for _, address := range strings.Fields(strings.Trim(value.Val, `"[]`)) {
			host, port, err := net.SplitHostPort(address)
			if err == nil && host == "127.0.0.1" && port != "0" {
				return address, nil
			}
		}
	}
	return "", fmt.Errorf("tor did not report a loopback SOCKS listener")
}

func stopTor(p *torProcess) error {
	if p == nil {
		return nil
	}
	if p.ctrl != nil {
		_ = p.ctrl.Signal("SHUTDOWN")
		_ = p.ctrl.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		if p.ctrl == nil {
			_ = p.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				_ = p.cmd.Process.Kill()
				<-p.done
			}
		} else {
			select {
			case <-p.done:
			case <-time.After(10 * time.Second):
				_ = p.cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-p.done:
				case <-time.After(5 * time.Second):
					_ = p.cmd.Process.Kill()
					<-p.done
				}
			}
		}
	}
	if p.eventCancel != nil {
		p.eventCancel()
	}
	if p.eventCtrl != nil {
		_ = p.eventCtrl.Close()
	}
	return nil
}
