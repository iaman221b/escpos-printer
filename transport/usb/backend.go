//go:build !windows

package usb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iaman221b/escpos-printer/device"
)

// writeTimeout bounds a write to a device node. A printer that has stopped
// draining its buffer would otherwise block the caller forever.
const writeTimeout = 5 * time.Second

// Backend writes ESC/POS bytes straight to a USB printer's device node.
//
// Deployment note: the device node is typically owned by root:lp, so the
// process must run as a user in the `lp` group (or a udev rule must grant
// access). Without it everything looks correctly configured and nothing prints
// — which is why the errors here name the fix rather than reporting a bare
// "permission denied".
type Backend struct {
	Path string
}

// NewBackend returns a Backend writing to the given device node path.
func NewBackend(path string) *Backend { return &Backend{Path: path} }

func (b *Backend) Name() string { return "usb:" + b.Path }

func (b *Backend) Print(ctx context.Context, data []byte) error {
	file, err := os.OpenFile(b.Path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("%w: could not open %s: %w", device.ErrDisconnected, b.Path, describeAccessError(err))
	}
	defer file.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = file.SetWriteDeadline(deadline)
	} else {
		_ = file.SetWriteDeadline(time.Now().Add(writeTimeout))
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("%w: could not write to %s: %w", device.ErrDisconnected, b.Path, err)
	}
	return nil
}

// Status checks that the node exists and can be opened for writing.
//
// Opening a usblp node succeeds only while the printer is attached and powered,
// so this is a genuine reachability check rather than a file-existence test.
//
// The node carries no paper signal, so this reports the link only. A real
// paper-out surfaces when a job fails, as it does for the other backends.
func (b *Backend) Status(ctx context.Context) (device.Status, error) {
	file, err := os.OpenFile(b.Path, os.O_WRONLY, 0)
	if err != nil {
		return device.Status{Online: false, Paper: device.PaperOut},
			fmt.Errorf("%w: could not open %s: %w", device.ErrDisconnected, b.Path, describeAccessError(err))
	}
	_ = file.Close()

	return device.Status{Online: true, Paper: device.PaperOK}, nil
}

// describeAccessError turns the usual permission failure into the sentence that
// actually fixes it. "permission denied" on its own has sent many hours down
// the wrong path.
func describeAccessError(err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("%w — the process needs access to the printer device (add its user to the 'lp' group, or add a udev rule)", err)
	}
	return err
}
