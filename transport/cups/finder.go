//go:build !windows

package cups

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/iaman221b/escpos-printer/device"
)

// Finder lists the print queues CUPS knows about, on Linux and macOS alike. A
// USB thermal printer installed as a queue appears here, as does any network
// printer CUPS has been given.
//
// Read-only: it runs `lpstat`, which reports and changes nothing.
type Finder struct{}

// NewFinder returns a CUPS queue finder.
func NewFinder() *Finder { return &Finder{} }

func (f *Finder) Name() string { return "cups" }

func (f *Finder) Find(ctx context.Context) ([]device.Discovered, error) {
	// -p lists queues with their state; -d appends the system default, which is
	// deliberately ignored — a system default is as likely to be a PDF writer
	// here as it is on Windows, and would silently swallow every receipt.
	cmd := exec.CommandContext(ctx, "lpstat", "-p")
	cmd.Env = cupsEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())

		// An empty spooler is an answer, not a fault: report no printers and
		// let the other finders speak.
		if noCUPSPrinters(err, message) {
			return nil, nil
		}

		// A genuine failure — cupsd unreachable, no permission to ask it. Not a
		// failure of discovery overall, since the network finder may still have
		// work to do, so this is reported and the registry moves on.
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("could not list CUPS printers (is CUPS installed and running?): %s", message)
	}

	var found []device.Discovered
	scanner := bufio.NewScanner(&stdout)

	for scanner.Scan() {
		name, enabled, ok := parseLpstatLine(scanner.Text())
		if !ok {
			continue
		}

		detail := "CUPS queue"
		if !enabled {
			detail = "CUPS queue — currently disabled"
		}
		if device.IsVirtualQueue(name) {
			detail = "Virtual printer — produces a document, not a receipt"
		}

		found = append(found, device.Discovered{
			Device: device.Device{
				ID:   device.QueueDeviceID("cups", name),
				Name: name,
				// CUPS hides the transport behind the queue. Most receipt
				// printers on a CUPS host are the USB one plugged into it; a
				// network printer is normally found by the network finder on
				// its own address instead.
				Connection: device.ConnectionUSB,
				Transport:  "cups",
				Receipt:    device.LooksLikeReceiptPrinter(name),
				Detail:     detail,
			},
			Backend: NewBackend(name),
		})
	}

	return found, scanner.Err()
}

// noCUPSPrinters reports whether lpstat's failure means "CUPS has nothing to
// list" rather than "CUPS is broken".
//
// Both are an empty result, but only one is worth a warning. A terminal that
// prints over the network, or a developer box with no queues added, has no
// reason to log a problem about a spooler it never uses — and a warning that
// appears on every healthy machine is a warning nobody reads on the one
// machine where it matters.
//
// The two quiet cases:
//
//   - lpstat is not installed at all, so there is no CUPS on this host.
//   - "lpstat: No destinations added." — CUPS is present and holds no queues.
//     Checked by message rather than exit status because builds disagree on it:
//     the same empty spooler exits non-zero before cupsd is up and zero after.
func noCUPSPrinters(err error, stderr string) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(stderr), "no destinations")
}

// parseLpstatLine pulls the queue name and enabled state out of a line like
//
//	printer TM-T82X-II is idle.  enabled since Tue 03 Feb 2026 09:12:01
//	printer Office-Laser disabled since Mon 02 Feb 2026 17:40:11 -
//
// Anything that is not a "printer <name> ..." line (blank lines, continuation
// text explaining why a queue stopped) is skipped.
func parseLpstatLine(line string) (name string, enabled bool, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "printer" {
		return "", false, false
	}

	lowered := strings.ToLower(line)
	return fields[1], !strings.Contains(lowered, "disabled"), true
}
