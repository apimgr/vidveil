// SPDX-License-Identifier: MIT
// AI.md PART 17: email test and template dry-run maintenance commands.
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/email"
)

type maintenanceEmailExitError struct {
	code int
	err  error
}

func (e maintenanceEmailExitError) Error() string { return e.err.Error() }
func (e maintenanceEmailExitError) Unwrap() error { return e.err }

func handleMaintenanceEmail(arg, configDir, dataDir string) error {
	fields := strings.Fields(arg)
	if len(fields) == 0 || fields[0] != "test" {
		return fmt.Errorf("usage: %s --maintenance email test [--to ADDRESS] [--dry-run]", filepath.Base(os.Args[0]))
	}
	to := ""
	dryRun := false
	for index := 1; index < len(fields); index++ {
		switch fields[index] {
		case "--dry-run":
			dryRun = true
		case "--to":
			if index+1 >= len(fields) {
				return fmt.Errorf("--to requires an email address")
			}
			index++
			to = fields[index]
		default:
			return fmt.Errorf("unknown email test option %q", fields[index])
		}
	}
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	service := email.NewEmailServiceAt(appConfig, config.GetAppPaths(configDir, dataDir).Config)
	if dryRun {
		previews, validationErrors := service.DryRunTemplates()
		for _, preview := range previews {
			fmt.Printf("Template: %s\nSubject: %s\n%s\n\n", preview.Name, preview.Subject, preview.Body)
		}
		if len(validationErrors) > 0 {
			for _, validationErr := range validationErrors {
				fmt.Fprintln(os.Stderr, validationErr)
			}
			return maintenanceEmailExitError{code: 65, err: fmt.Errorf("email template validation failed")}
		}
		return nil
	}
	if to == "" {
		to = strings.TrimSpace(appConfig.Server.Notifications.Email.ReplyTo)
	}
	if to == "" {
		return fmt.Errorf("recipient required: set server.notifications.email.reply_to or pass --to ADDRESS")
	}
	host, port, tlsMode, warnings := email.ResolveSMTPSettings(appConfig.Server.Notifications.Email.SMTP.Host, appConfig.Server.Notifications.Email.SMTP.Port, appConfig.Server.Notifications.Email.SMTP.TLS)
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "WARN:", warning)
	}
	if host == "" {
		host, port = email.AutodetectSMTP(email.SMTPAutodetectHosts(appConfig.Server.FQDN), nil)
	}
	if err := email.TestSMTPConfigMode(host, port, tlsMode); err != nil {
		return fmt.Errorf("SMTP is unavailable: %w", err)
	}
	appConfig.Server.Notifications.Email.SMTP.Host = host
	appConfig.Server.Notifications.Email.SMTP.Port = port
	appConfig.Server.Notifications.Email.Enabled = true
	logger, closeLogger, err := newCLIAuditLogger(appConfig, configDir, dataDir)
	if err != nil {
		return fmt.Errorf("initialize audit log: %w", err)
	}
	defer closeLogger()
	subject := "[TEST] Test Email - " + appConfig.Server.Branding.Title
	fmt.Printf("Sending to: %s\nTemplate: test\nSubject: %s\n", to, subject)
	sendErr := service.SendTest(to)
	result := "success"
	if sendErr != nil {
		result = "failure"
	}
	logger.Audit("email.test", "cli", "operator", "", result, map[string]interface{}{"recipient": to, "template": "test", "subject": subject})
	if sendErr != nil {
		return fmt.Errorf("send test email: %w", sendErr)
	}
	return nil
}

func emailExitCode(err error) int {
	var exitError maintenanceEmailExitError
	if errors.As(err, &exitError) {
		return exitError.code
	}
	return 1
}
