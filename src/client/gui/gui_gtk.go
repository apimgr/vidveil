// SPDX-License-Identifier: MIT
// AI.md PART 32: GTK4 GUI launcher for Linux and BSD systems.

//go:build (linux || freebsd || openbsd || netbsd) && gui

package gui

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func launchGTKGui(cfg *Config) error {
	app := gtk.NewApplication("io.github.apimgr.vidveil.cli", gio.ApplicationFlagsNone)

	app.ConnectActivate(func() {
		win := gtk.NewApplicationWindow(app)
		win.SetTitle(cfg.BinaryName)
		win.SetDefaultSize(800, 600)
		buildMainWindow(win, cfg)
		win.Show()
	})

	if code := app.Run(nil); code != 0 {
		return ErrGUIUnsupported
	}
	return nil
}

func buildMainWindow(win *gtk.ApplicationWindow, cfg *Config) {
	box := gtk.NewBox(gtk.OrientationVertical, 10)
	box.SetMarginTop(10)
	box.SetMarginBottom(10)
	box.SetMarginStart(10)
	box.SetMarginEnd(10)

	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)
	win.SetTitlebar(header)

	label := gtk.NewLabel("Connected to: " + cfg.ServerURL)
	box.Append(label)

	win.SetChild(box)
}
