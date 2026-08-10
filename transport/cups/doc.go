// Package cups reaches printers through CUPS, the print system both Linux and
// macOS use.
//
// A USB thermal printer shows up as an ordinary CUPS queue once installed, so
// this is the USB path on macOS as well as the route to any queue CUPS holds.
// On macOS it is the only route: there is no /dev/usb/lp* device node there.
//
// The implementation is behind a "!windows" build tag; this file carries no tag
// so that the package still exists (and `go build ./...` still succeeds) when
// compiling for Windows.
package cups
