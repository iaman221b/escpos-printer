//go:build windows

package windows

import (
	"context"
	"log/slog"
	"strings"

	"github.com/iaman221b/escpos-printer/device"
)

// Finder lists the machine's Windows print queues.
//
// A USB thermal printer does not appear as a device file on Windows — it is
// installed as an ordinary print queue on a USB port, which is why the spooler
// is the supported path for USB here and why this finder covers both USB and
// any queue pointing at a network printer.
//
// Read-only: it asks Windows what exists and changes nothing about how the
// spooler backend prints.
type Finder struct {
	// Logger receives a line for each queue skipped as offline. Nil means
	// slog.Default().
	Logger *slog.Logger
}

// NewFinder returns a Windows spooler queue finder.
func NewFinder() *Finder { return &Finder{} }

func (f *Finder) Name() string { return "windows-spooler" }

func (f *Finder) logger() *slog.Logger {
	if f.Logger != nil {
		return f.Logger
	}
	return slog.Default()
}

func (f *Finder) Find(ctx context.Context) ([]device.Discovered, error) {
	var found []device.Discovered

	for _, queue := range ListQueues() {
		name := strings.TrimSpace(queue)
		if name == "" {
			continue
		}
		// ListQueues reports enumeration trouble in-band as "<enumeration
		// failed: ...>" rather than as an error. Not a printer.
		if strings.HasPrefix(name, "<") {
			continue
		}

		virtual := device.IsVirtualQueue(name)

		// Skip disconnected printers: a settings screen should list what is
		// actually reachable, not every queue Windows remembers from a previous
		// connection. Virtual printers are kept regardless — they are always
		// technically reachable and operators need to see them to understand
		// what was considered and rejected.
		if !virtual {
			if online, reason := QueueOnline(name); !online {
				f.logger().Info("skipping offline queue during discovery",
					"queue", name, "reason", reason)
				continue
			}
		}

		detail := "Windows print queue"
		if virtual {
			// Surfaced rather than hidden: an operator who cannot find their
			// printer needs to see that the thing named "Microsoft Print to
			// PDF" was considered and rejected.
			detail = "Virtual printer — produces a document, not a receipt"
		}

		found = append(found, device.Discovered{
			Device: device.Device{
				ID:         device.QueueDeviceID("windows", name),
				Name:       name,
				Connection: device.ConnectionUSB,
				Transport:  "windows-spooler",
				Receipt:    device.LooksLikeReceiptPrinter(name),
				Detail:     detail,
			},
			Backend: NewSpoolerBackend(name),
		})
	}

	return found, nil
}
