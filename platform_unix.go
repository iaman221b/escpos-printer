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

// configuredLocalDevice resolves an explicitly configured local queue.
//
// "cups" names a queue; "usb"/"device" names a device node path. A mode
// belonging to another platform — "windows", say — is ignored rather than
// treated as an error, so one shared configuration can serve a mixed fleet
// without the Unix hosts refusing to start.
func configuredLocalDevice(mode, name string) (device.Discovered, bool) {
	switch mode {
	case "cups":
		return device.Discovered{
			Device: device.Device{
				ID:         device.QueueDeviceID("cups", name),
				Name:       name,
				Connection: device.ConnectionUSB,
				Transport:  "cups",
				// Named by an operator, so treated as intended for receipts.
				Receipt: true,
				Detail:  "Named in the application's configuration",
			},
			Backend: cups.NewBackend(name),
		}, true

	case "usb", "device":
		return device.Discovered{
			Device: device.Device{
				ID:         "usb:" + name,
				Name:       name,
				Connection: device.ConnectionUSB,
				Transport:  "usb-device",
				Receipt:    true,
				Detail:     "Named in the application's configuration",
			},
			Backend: usb.NewBackend(name),
		}, true

	default:
		return device.Discovered{}, false
	}
}
