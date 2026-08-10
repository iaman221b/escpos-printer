//go:build !windows

package escposprinter

import (
	"log/slog"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/transport/cups"
	"github.com/iaman221b/escpos-printer/transport/usb"
)

// platformFinders returns the printer sources that exist on this platform.
//
// On Linux and macOS that is CUPS (the queues the operating system knows about)
// plus a sweep of USB device nodes, which catches the thermal printers that
// ship no CUPS driver and so never become a queue. Both are needed: neither
// alone covers every USB receipt printer, and on macOS only CUPS finds one.
//
// This file and its Windows counterpart are the entire platform surface of the
// library. Go selects between them at compile time.
func platformFinders(logger *slog.Logger) []device.Finder {
	return []device.Finder{
		cups.NewFinder(),
		usb.NewFinder(),
	}
}
