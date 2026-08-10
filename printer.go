package escposprinter

import (
	"context"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/escpos"
)

// Printer is a handle to one discovered printer: what it is, and how to reach
// it. Obtain one from Registry.Active or Registry.Printer.
//
// A Printer is safe for concurrent use to the extent its backend is; the
// backends in this library open a fresh connection per operation, so concurrent
// jobs are serialised by the device rather than by this type.
type Printer struct {
	dev     device.Device
	backend device.Backend
}

// NewPrinter wraps a device and its backend directly, for a printer the caller
// already knows about and does not want discovered.
func NewPrinter(dev device.Device, backend device.Backend) *Printer {
	return &Printer{dev: dev, backend: backend}
}

// Device describes this printer in terms an operator can recognise.
func (p *Printer) Device() device.Device { return p.dev }

// ID is the stable identifier for this printer, unchanged across restarts.
func (p *Printer) ID() string { return p.dev.ID }

// Print sends already-rendered ESC/POS bytes to the device.
//
// Build the bytes with the escpos package:
//
//	data := escpos.NewBuilder().Init().Line("TOTAL 25.00").Cut().Bytes()
//	err := p.Print(ctx, data)
//
// Failures wrap ErrDisconnected or ErrPaperOut.
func (p *Printer) Print(ctx context.Context, data []byte) error {
	return p.backend.Print(ctx, data)
}

// Status probes the device without printing, for a diagnostics screen or a
// pre-sale check.
//
// A probe failure is reported through the error while the Status is still
// returned: "could not reach the printer" is an answer, and the DeviceID it
// belongs to is still known. How much Status can say depends on the transport —
// only the Windows spooler exposes a real paper signal.
func (p *Printer) Status(ctx context.Context) (device.Status, error) {
	return p.backend.Status(ctx)
}

// OpenDrawer fires the cash drawer's solenoid on its own, for the cases not
// tied to a receipt: a settings screen's Test button, a no-sale open.
//
// To open the drawer *with* a receipt, prepend the kick to the receipt's own
// byte stream instead — see escpos.Builder.DrawerKick. A printer processes one
// job at a time, so putting both in one job makes "drawer opens before the
// receipt prints" a property of the byte order rather than of scheduling.
//
// A failure here means the pulse could not be delivered to the printer. It
// never means the drawer failed to open: the DK port has no read-back, so a
// printer accepts the kick identically whether or not a drawer is attached.
// Nothing in this library can observe the drawer's mechanism, and none of it
// pretends to.
func (p *Printer) OpenDrawer(ctx context.Context, pulse escpos.DrawerPulse) error {
	return p.backend.Print(ctx, pulse.Normalize().ToBytes())
}

// Backend exposes the underlying transport, for callers that need something
// this API does not wrap.
func (p *Printer) Backend() device.Backend { return p.backend }
