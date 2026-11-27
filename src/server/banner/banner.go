// SPDX-License-Identifier: MIT
// Package banner prints the server startup banner independently from client banners.
package banner

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/apimgr/vidveil/src/common/display"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/terminal"
)

// ServerBannerInfo contains the resolved values shown in the server startup banner.
type ServerBannerInfo struct {
	AppName         string
	Version         string
	AppMode         string
	StartupDatetime string
	URLs            []string
	OnionAddress    string
	I2PAddress      string
	SMTPAddress     string
	SetupToken      string
	ForceColor      *bool
	Lang            string
}

// PrintServerStartupBanner prints a banner sized from terminal columns only.
func PrintServerStartupBanner(info ServerBannerInfo) {
	displayEnv := display.DetectDisplayEnv()
	width := displayEnv.Cols
	if width <= 0 {
		width, _ = strconv.Atoi(os.Getenv("COLUMNS"))
		if width <= 0 {
			width = 80
		}
	}
	color := terminal.ColorEnabled()
	if info.ForceColor != nil {
		color = *info.ForceColor
	}
	emojis := terminal.EmojiEnabled() && (info.ForceColor == nil || *info.ForceColor)
	if !emojis {
		fmt.Printf("%s v%s\n%s: %s\n", info.AppName, info.Version, i18n.Translate(info.Lang, "cli.running_in_mode_label"), info.AppMode)
		printDetails(info, false)
		fmt.Println()
		return
	}
	switch modeForWidth(width) {
	case 3:
		fmt.Println(serverASCIIArt(info.AppName))
		fmt.Printf("🚀 %s v%s\n🔒 %s: %s\n", colorize(info.AppName, color), info.Version, i18n.Translate(info.Lang, "cli.running_in_mode_label"), info.AppMode)
		printDetails(info, true)
	case 2:
		fmt.Printf("🚀 %s v%s\n🔒 %s: %s\n", colorize(info.AppName, color), info.Version, i18n.Translate(info.Lang, "cli.running_in_mode_label"), info.AppMode)
		printDetails(info, true)
	case 1:
		fmt.Printf("%s %s %s\n", info.AppName, info.Version, compactAddress(info.URLs))
		if info.SetupToken != "" {
			fmt.Printf("Setup token: %s\n", info.SetupToken)
		}
	default:
		fmt.Printf("%s %s\n", info.AppName, compactAddress(info.URLs))
		if info.SetupToken != "" {
			fmt.Printf("Setup token: %s\n", info.SetupToken)
		}
	}
	if color {
		fmt.Print("\033[0m")
	}
}

func printDetails(info ServerBannerInfo, icons bool) {
	for _, address := range info.URLs {
		if icons {
			fmt.Printf("📡 Listening on %s\n", address)
		} else {
			fmt.Printf("Listening on %s\n", address)
		}
	}
	if info.OnionAddress != "" {
		prefix := "Tor: "
		if icons {
			prefix = "🧅 Tor: "
		}
		fmt.Printf("%shttp://%s\n", prefix, info.OnionAddress)
	}
	if info.I2PAddress != "" {
		prefix := "I2P: "
		if icons {
			prefix = "🔗 I2P: "
		}
		fmt.Printf("%shttp://%s\n", prefix, info.I2PAddress)
	}
	if info.SMTPAddress != "" {
		prefix := "SMTP: "
		if icons {
			prefix = "📧 SMTP: "
		}
		fmt.Printf("%s%s\n", prefix, info.SMTPAddress)
	}
	if info.StartupDatetime != "" {
		prefix := "Server started on "
		if icons {
			prefix = "✅ " + prefix
		}
		fmt.Printf("%s%s\n", prefix, info.StartupDatetime)
	}
	if info.SetupToken != "" {
		prefix := "Setup token: "
		if icons {
			prefix = "🔑 " + prefix
		}
		fmt.Printf("%s%s\n", prefix, info.SetupToken)
	}
}

func modeForWidth(width int) int {
	switch {
	case width >= 80:
		return 3
	case width >= 60:
		return 2
	case width >= 40:
		return 1
	default:
		return 0
	}
}

func colorize(value string, enabled bool) string {
	if !enabled {
		return value
	}
	return "\033[1;36m" + value + "\033[0m"
}

func serverASCIIArt(appName string) string {
	if strings.EqualFold(appName, "vidveil") {
		return `
 __      ___     ___      __    _ _
 \ \    / (_)   | \ \    / /   (_) |
  \ \  / / _  __| |\ \  / /__  _| |
   \ \/ / | |/ _` + "`" + ` | \ \/ / _ \| | |
    \  /  | | (_| |  \  /  __/| | |
     \/   |_|\__,_|   \/ \___||_|_|`
	}
	return "=== " + strings.ToUpper(appName) + " ==="
}

func compactAddress(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	address := urls[0]
	if index := strings.Index(address, "://"); index >= 0 {
		address = address[index+3:]
	}
	if index := strings.IndexByte(address, '/'); index >= 0 {
		address = address[:index]
	}
	return address
}
