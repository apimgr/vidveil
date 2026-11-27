// SPDX-License-Identifier: MIT
// AI.md PART 11: operator maintenance for coordinated security reports.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

func handleMaintenanceSecurity(arg, configDir, dataDir string) error {
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return errors.New("usage: --maintenance security <list|show|export|update>")
	}
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open security report database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := mgr.GetAppDatabase()
	switch fields[0] {
	case "list":
		status := ""
		if len(fields) != 1 {
			if len(fields) != 3 || fields[1] != "--status" {
				return errors.New("usage: --maintenance security list [--status STATUS]")
			}
			status = fields[2]
		}
		reports, err := secreport.ListReports(ctx, db, status)
		if err != nil {
			return err
		}
		fmt.Printf("%-24s %-12s %-14s %-24s %-20s %s\n", "TRACKING_ID", "STATUS", "SEVERITY", "COMPONENT", "CREATED_AT", "UPDATED_AT")
		for _, report := range reports {
			fmt.Printf("%-24s %-12s %-14s %-24q %-20s %s\n", report.TrackingID, report.Status, report.Severity, report.Component, report.CreatedAt.UTC().Format(time.RFC3339), report.UpdatedAt.UTC().Format(time.RFC3339))
		}
		return nil
	case "show":
		if len(fields) != 2 {
			return errors.New("usage: --maintenance security show TRACKING_ID")
		}
		report, err := secreport.GetReportDetails(ctx, db, fields[1])
		if err != nil {
			return err
		}
		fmt.Printf("Tracking ID: %s\nStatus: %s\nSeverity: %s\nComponent: %q\nCreated: %s\nUpdated: %s\nResearcher comment: %q\n", report.TrackingID, report.Status, report.Severity, report.Component, report.CreatedAt.UTC().Format(time.RFC3339), report.UpdatedAt.UTC().Format(time.RFC3339), report.ResearcherComment)
		if report.DisclosureDate.Valid {
			fmt.Printf("Disclosure date: %s\n", report.DisclosureDate.Time.UTC().Format("2006-01-02"))
		} else {
			fmt.Println("Disclosure date: (not set)")
		}
		return nil
	case "export":
		if len(fields) != 2 && len(fields) != 4 || len(fields) == 4 && fields[2] != "--output" {
			return errors.New("usage: --maintenance security export TRACKING_ID [--output FILE]")
		}
		trackingID := fields[1]
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			return fmt.Errorf("load security notification settings: %w", err)
		}
		previousKey, valid, err := secret.LoadPreviousEncryptionKey(ctx, db, appConfig.Server.Security.EncryptionKey, time.Now().UTC())
		if err != nil {
			return err
		}
		if !valid {
			previousKey = ""
		}
		body, algorithm, err := secreport.EncryptedBodyWithKeys(ctx, db, trackingID, appConfig.Server.Security.EncryptionKey, previousKey)
		if err != nil {
			_ = logSecurityMaintenance(configDir, dataDir, "security.decrypt_failed", trackingID, map[string]interface{}{"table": "security_reports"})
			return err
		}
		if algorithm == secreport.EncryptionMethodAES {
			report, err := secreport.GetReportDetails(ctx, db, trackingID)
			if err != nil {
				return err
			}
			warning := "The security report is attached as AES-256-GCM ciphertext. Decrypt it with the configured server.security.encryption_key. The attachment contains no plaintext fallback."
			from := appConfig.Server.Notifications.Email.From
			fromAddr := from.Email
			if fromAddr == "" {
				fromAddr = "no-reply@" + appConfig.Server.FQDN
			}
			fromName := from.Name
			if fromName == "" {
				fromName = appConfig.Server.Branding.Title
			}
			to := appConfig.Server.Contact.Security.Email
			if to == "" {
				to = appConfig.Server.Contact.Admin.Email
			}
			subject := fmt.Sprintf("[Security Report] %s - %s (%s)", report.Severity, report.Component, trackingID)
			filename := "security-report-" + trackingID + ".aesgcm"
			body, err = email.BuildRawAttachmentMessage(fromName, fromAddr, to, subject, warning, filename, "application/octet-stream", body)
			if err != nil {
				return fmt.Errorf("build encrypted report export message: %w", err)
			}
		}
		output := trackingID + ".asc"
		if len(fields) == 4 {
			output = fields[3]
		}
		if output == "-" {
			if _, err := os.Stdout.Write(body); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(output, body, 0600); err != nil {
				return fmt.Errorf("write encrypted report export: %w", err)
			}
			if err := os.Chmod(output, 0600); err != nil {
				return fmt.Errorf("secure encrypted report export: %w", err)
			}
		}
		if err := logSecurityMaintenance(configDir, dataDir, "security.report_exported", trackingID, map[string]interface{}{"body_alg": string(algorithm)}); err != nil {
			return err
		}
		return nil
	case "update":
		if len(fields) < 2 {
			return errors.New("usage: --maintenance security update TRACKING_ID [--status STATUS] [--comment TEXT] [--disclosure-date YYYY-MM-DD] [--severity LEVEL]")
		}
		update := secreport.UpdateInput{}
		for i := 2; i < len(fields); i++ {
			option := fields[i]
			if i+1 >= len(fields) {
				return fmt.Errorf("missing value for %s", fields[i])
			}
			value := fields[i+1]
			switch fields[i] {
			case "--status":
				update.Status = &value
			case "--comment":
				j := i + 2
				for j < len(fields) && !strings.HasPrefix(fields[j], "--") {
					j++
				}
				value = strings.Join(fields[i+1:j], " ")
				update.Comment = &value
				i = j - 2
			case "--severity":
				update.Severity = &value
			case "--disclosure-date":
				date, err := time.Parse("2006-01-02", value)
				if err != nil {
					return fmt.Errorf("invalid disclosure date %q", value)
				}
				update.DisclosureDate = &date
			default:
				return fmt.Errorf("unknown security update option %q", fields[i])
			}
			if option != "--comment" {
				i++
			}
		}
		result, err := secreport.UpdateReport(ctx, db, fields[1], update, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := logSecurityMaintenance(configDir, dataDir, "security.report_updated", fields[1], map[string]interface{}{"old_status": result.OldStatus, "new_status": result.NewStatus}); err != nil {
			return err
		}
		appConfig, _, configErr := config.LoadAppConfig(configDir, dataDir)
		if configErr == nil {
			previousKey, valid, _ := secret.LoadPreviousEncryptionKey(ctx, db, appConfig.Server.Security.EncryptionKey, time.Now().UTC())
			if !valid {
				previousKey = ""
			}
			if contact, contactErr := secreport.GetContactWithPrevious(ctx, db, fields[1], appConfig.Server.Security.EncryptionKey, previousKey); contactErr == nil && contact.Email != "" {
				notifier := email.NewEmailService(appConfig)
				body := "The status of your coordinated-disclosure report has changed.\n\nTracking ID: " + fields[1] + "\nStatus: " + result.NewStatus + "\n"
				_ = notifier.SendRaw(contact.Email, "Security report status updated - "+fields[1], body)
			} else if contactErr != nil && !strings.Contains(contactErr.Error(), "has no contact details") {
				_ = logSecurityMaintenance(configDir, dataDir, "security.decrypt_failed", fields[1], map[string]interface{}{"table": "security_reports", "column": "contact_enc"})
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown security report action %q", fields[0])
	}
}

func logSecurityMaintenance(configDir, dataDir, event, trackingID string, details map[string]interface{}) error {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("load config for security audit event: %w", err)
	}
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		return fmt.Errorf("open security audit log: %w", err)
	}
	defer logger.Close()
	details["tracking_id"] = trackingID
	logger.Security(event, "", details)
	return nil
}
