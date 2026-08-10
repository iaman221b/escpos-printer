//go:build windows

package escposprinter

import (
	"log/slog"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/transport/windows"
)

// platformFinders returns the printer sources that exist on this platform.
//
// On Windows that is the print spooler, which covers both USB thermal printers
// (installed as a queue on a USB port) and any queue pointing at a network
// printer. Printers with their own address are found separately by the
// platform-independent network finder, when the caller enables it.
//
// This file and its Unix counterpart are the entire platform surface of the
// library. Go selects between them at compile time, so the Windows syscall code
// is never linked into a Linux or macOS binary.
func platformFinders(logger *slog.Logger) []device.Finder {
	return []device.Finder{
		&windows.Finder{Logger: logger},
	}
}
