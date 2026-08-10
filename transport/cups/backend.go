//go:build !windows

package cups

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/iaman221b/escpos-printer/device"
)

// Backend prints through CUPS. One implementation covers Linux and macOS,
// which is why this file is tagged "not Windows" rather than per-OS.
//
// The critical detail is the raw datatype. By default CUPS runs a job through a
// filter chain that renders it as a page — which turns ESC/POS control bytes
// into either garbage or nothing at all. `-o raw` (equivalent to the
// application/vnd.cups-raw MIME type) tells CUPS to hand the bytes to the
// device untouched, exactly as the Windows spooler's RAW datatype does.
type Backend struct {
	QueueName string
}

// NewBackend returns a Backend for the named CUPS queue.
func NewBackend(queueName string) *Backend {
	return &Backend{QueueName: queueName}
}

func (b *Backend) Name() string { return "cups:" + b.QueueName }

// cupsEnv forces CUPS tools to answer in English.
//
// lpstat's output is localised: on a machine set to another language the state
// words this package looks for ("idle", "disabled", "enabled") are translated,
// and a perfectly healthy printer would be reported as unreachable. Pinning
// LC_ALL for the child process alone costs nothing and removes the whole class
// of failure.
func cupsEnv() []string {
	return append(os.Environ(), "LC_ALL=C", "LANG=C")
}

func (b *Backend) Print(ctx context.Context, data []byte) error {
	// -d  target queue          (never the system default: that is as likely to
	//                            be a PDF writer here as it is on Windows)
	// -o raw  pass bytes through unfiltered
	// -      read the job from stdin
	cmd := exec.CommandContext(ctx, "lp", "-d", b.QueueName, "-o", "raw", "-")
	cmd.Env = cupsEnv()
	cmd.Stdin = bytes.NewReader(data)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%w: lp failed for %q: %s", device.ErrDisconnected, b.QueueName, message)
	}

	return nil
}

// Status asks CUPS whether the queue is enabled and accepting work.
//
// CUPS reports a queue as disabled when the device stops answering, so this
// covers an unplugged USB printer. It cannot see paper level — no queue-level
// mechanism exposes it — so paper is reported as OK and a genuine paper-out is
// discovered when a job fails, exactly as on the network backend.
func (b *Backend) Status(ctx context.Context) (device.Status, error) {
	cmd := exec.CommandContext(ctx, "lpstat", "-p", b.QueueName)
	cmd.Env = cupsEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return device.Status{Online: false, Paper: device.PaperOut},
			fmt.Errorf("%w: could not read status for %q: %s", device.ErrDisconnected, b.QueueName, message)
	}

	// lpstat answers in prose: "printer TM-T82X-II is idle. enabled since ..."
	// or "printer TM-T82X-II disabled since ...". The state word is what
	// matters; the rest is a timestamp. cupsEnv pins the language so these
	// words are stable.
	report := strings.ToLower(stdout.String())
	switch {
	case strings.Contains(report, "disabled"):
		return device.Status{Online: false, Paper: device.PaperOut},
			fmt.Errorf("%w: queue %q is disabled — CUPS stopped it, usually because the device stopped answering",
				device.ErrDisconnected, b.QueueName)
	case strings.Contains(report, "idle"), strings.Contains(report, "printing"), strings.Contains(report, "enabled"):
		return device.Status{Online: true, Paper: device.PaperOK}, nil
	}

	// An unrecognised reply is not a claim of health.
	return device.Status{Online: false, Paper: device.PaperOut},
		fmt.Errorf("%w: could not interpret the CUPS status for %q: %s",
			device.ErrDisconnected, b.QueueName, strings.TrimSpace(stdout.String()))
}
