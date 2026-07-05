// SPDX-License-Identifier: MIT
// AI.md PART 32: Cocoa GUI launcher for macOS.

//go:build darwin && gui

package gui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void launchCocoaApp(const char* title) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

    NSRect frame = NSMakeRect(0, 0, 800, 600);
    NSWindow *window = [[NSWindow alloc]
        initWithContentRect:frame
        styleMask:(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                   NSWindowStyleMaskResizable | NSWindowStyleMaskMiniaturizable)
        backing:NSBackingStoreBuffered
        defer:NO];

    NSString *titleStr = [NSString stringWithUTF8String:title];
    [window setTitle:titleStr];
    [window center];
    [window makeKeyAndOrderFront:nil];

    [NSApp activateIgnoringOtherApps:YES];
    [NSApp run];
}
*/
import "C"
import "unsafe"

func launchCocoaGui(cfg *Config) error {
	title := C.CString(cfg.BinaryName)
	defer C.free(unsafe.Pointer(title))
	C.launchCocoaApp(title)
	return nil
}
