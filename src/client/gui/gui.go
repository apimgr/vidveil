// SPDX-License-Identifier: MIT
// AI.md PART 32: GUI mode — compiled only when -tags gui is provided.
// GUI uses github.com/gogpu/ui (+ github.com/gogpu/gogpu for windowing), a
// pure-Go, zero-CGO GPU-accelerated widget toolkit (goffi dlopens the native
// windowing libs at runtime, no link-time C dependency). One implementation
// covers Linux (X11/Wayland), macOS, and Windows unmodified — there is no
// per-OS launcher file and no runtime.GOOS dispatch. gogpu/gogpu's windowing
// layer does not implement BSD yet, so this file is excluded there in favor
// of gui_unsupported.go (build-tag gated), which falls back to TUI/CLI.

//go:build gui && !freebsd && !netbsd && !openbsd

package gui

import (
	"log"
	"strconv"
	"strings"

	"github.com/gogpu/gg"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"

	"github.com/apimgr/vidveil/src/common/display"
	"github.com/apimgr/vidveil/src/common/theme"
)

// Config holds the configuration passed to the GUI launcher.
type Config struct {
	ServerURL  string
	Token      string
	Version    string
	BinaryName string
}

// IsAvailable reports whether a native GUI can be launched in the current environment.
// Remote sessions (SSH/Mosh) never have GUI even when DISPLAY is set.
func IsAvailable() bool {
	env := display.DetectDisplayEnv()
	return env.HasDisplay && !env.IsSSH && !env.IsMosh
}

// Launch opens the gogpu/ui window. gogpu/ui is pure Go, zero CGO (via
// goffi's runtime dlopen) and cross-platform — the same code runs unmodified
// on Linux (X11/Wayland), macOS, and Windows. BSD has no backend yet, so
// IsAvailable/caps.GUISupported must already be false there and this
// function must never be reached on freebsd/netbsd/openbsd.
func Launch(cfg *Config) error {
	title := cfg.BinaryName
	if title == "" {
		title = "vidveil-cli"
	}

	gogpuApp := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(title).
		WithSize(800, 600))

	m3 := buildMaterial3Theme()

	uiApp := app.New(
		app.WithWindowProvider(gogpuApp),
		app.WithPlatformProvider(gogpuApp),
		app.WithEventSource(gogpuApp.EventSource()),
	)
	uiApp.SetRoot(buildMainWindow(cfg, m3))

	var canvas *ggcanvas.Canvas
	gogpuApp.OnDraw(func(dc *gogpu.Context) {
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}
		if canvas == nil {
			provider := gogpuApp.GPUContextProvider()
			if provider == nil {
				return
			}
			var err error
			canvas, err = ggcanvas.New(provider, w, h)
			if err != nil {
				log.Printf("ggcanvas: %v", err)
				return
			}
		}
		uiApp.Frame()
		cw, ch := canvas.Size()
		if cw != w || ch != h {
			if err := canvas.Resize(w, h); err != nil {
				log.Printf("resize: %v", err)
			}
			cw, ch = w, h
		}
		// dc.SurfaceView() returns the concrete *wgpu.TextureView; ggcanvas
		// wants the gpucontext-opaque handle, which the RenderTarget adapter
		// provides directly (see gogpu.ContextRenderTarget.SurfaceView).
		sv := dc.RenderTarget().SurfaceView()
		sw, sh := dc.SurfaceSize()
		canvas.Draw(func(cc *gg.Context) {
			cc.SetRGBA(0.94, 0.94, 0.94, 1)
			cc.DrawRectangle(0, 0, float64(cw), float64(ch))
			cc.Fill()
			uiApp.Window().DrawTo(render.NewCanvas(cc, cw, ch))
		})
		if err := canvas.RenderDirect(sv, sw, sh); err != nil {
			log.Printf("render: %v", err)
		}
	})

	return gogpuApp.Run()
}

// buildMaterial3Theme resolves gogpu/ui's Material 3 theme from the
// project's own OS light/dark detection (gogpu/ui has none) and seeds it
// from the project's canonical ColorPalette — never a literal hex value
// invented here (see Color Palette rule; CLI/TUI/GUI Theming, PART 32/16).
func buildMaterial3Theme() *material3.Theme {
	name := theme.GetColorPaletteName("auto")
	palette := theme.GetColorPalette(name)
	seed := hexToWidgetColor(palette.Primary)
	if name == "dark" {
		return material3.NewDark(seed)
	}
	return material3.New(seed)
}

// hexToWidgetColor converts a "#rrggbb" ColorPalette string into the color
// value gogpu/ui's widget package expects, falling back to opaque black on
// a malformed value (should never happen — palette values are compile-time
// constants in src/common/theme).
func hexToWidgetColor(hex string) widget.Color {
	h := strings.TrimPrefix(hex, "#")
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return widget.Hex(0x000000)
	}
	return widget.Hex(uint32(v))
}

// buildMainWindow renders the main window content.
func buildMainWindow(cfg *Config, m3 *material3.Theme) *primitives.BoxWidget {
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	server := cfg.ServerURL
	if server == "" {
		server = "(no server configured)"
	}

	// Content area — extend based on CLI functionality (PART 32).
	return primitives.Box(
		primitives.Text(cfg.BinaryName).FontSize(24).Bold(),
		primitives.Text("Version "+version).FontSize(14),
		primitives.Text("Server: "+server).FontSize(14),
	).Padding(24).Gap(12)
}
