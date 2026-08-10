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

// configuredLocalDevice resolves an explicitly configured local queue.
//
// This is the path a working Windows till takes: mode "windows" plus the queue
// name produces the very same spooler backend, under the very same device ID
// the Windows finder would produce for that queue, so the two agree and the
// configured printer is pinned active.
//
// A mode belonging to another platform is ignored rather than treated as an
// error, so one shared configuration can serve a mixed fleet.
func configuredLocalDevice(mode, name string) (device.Discovered, bool) {
	if mode != "windows" && mode != "spooler" {
		return device.Discovered{}, false
	}

	return device.Discovered{
		Device: device.Device{
			ID:         device.QueueDeviceID("windows", name),
			Name:       name,
			Connection: device.ConnectionUSB,
			Transport:  "windows-spooler",
			// Named by an operator, so treated as intended for receipts.
			Receipt: true,
			Detail:  "Named in the application's configuration",
		},
		Backend: windows.NewSpoolerBackend(name),
	}, true
}
