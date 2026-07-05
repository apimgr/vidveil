// SPDX-License-Identifier: MIT
// AI.md PART 32: Win32 GUI launcher for Windows (pure Go, no CGO).

//go:build windows && gui

package gui

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	swShow             = 5
	wmDestroy          = 0x0002
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	createWindowExW  = user32.NewProc("CreateWindowExW")
	defWindowProcW   = user32.NewProc("DefWindowProcW")
	dispatchMessageW = user32.NewProc("DispatchMessageW")
	getMessageW      = user32.NewProc("GetMessageW")
	postQuitMessage  = user32.NewProc("PostQuitMessage")
	registerClassExW = user32.NewProc("RegisterClassExW")
	showWindow       = user32.NewProc("ShowWindow")
	translateMessage = user32.NewProc("TranslateMessage")
	updateWindow     = user32.NewProc("UpdateWindow")
)

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

func wndProc(hwnd, wm, wParam, lParam uintptr) uintptr {
	if wm == wmDestroy {
		postQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := defWindowProcW.Call(hwnd, wm, wParam, lParam)
	return ret
}

func launchWin32Gui(cfg *Config) error {
	className := windows.StringToUTF16Ptr("vidveil_cli_window")
	windowTitle := windows.StringToUTF16Ptr(cfg.BinaryName)

	var wc wndClassExW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(wndProc)
	wc.LpszClassName = className

	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		wsOverlappedWindow|wsVisible,
		100, 100, 800, 600,
		0, 0, 0, 0,
	)

	showWindow.Call(hwnd, swShow)
	updateWindow.Call(hwnd)

	var m msg
	for {
		ret, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	return nil
}
