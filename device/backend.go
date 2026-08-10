package device

import "context"

// PaperStatus is what a status probe could learn about the paper roll.
type PaperStatus string

const (
	PaperOK  PaperStatus = "ok"
	PaperLow PaperStatus = "low"
	PaperOut PaperStatus = "out"
)

// Status is the result of a printer health probe.
//
// Every backend in this library talks to a real device. There is deliberately
// no mock that answers Online without anything being plugged in — "no printer"
// is the absence of a backend (see ErrNoPrinter), not a backend that pretends.
// A fake that reports itself healthy makes every diagnostics screen lie, which
// is worse than having no diagnostics at all.
type Status struct {
	// Online reports whether the device answered.
	Online bool

	// Paper is the roll state where the transport can see it.
	//
	// Not every transport can. The Windows spooler exposes real paper bits; a
	// CUPS queue and a USB device node expose nothing, and report PaperOK
	// because they have no signal either way. A genuine paper-out on those
	// transports surfaces when a print job fails.
	Paper PaperStatus
}

// Backend is one concrete way of talking to one printer: a Windows spooler
// queue, a CUPS queue, a USB device node, a TCP socket.
//
// Everything above this interface is transport-agnostic. That is what lets a
// single rendered byte stream reach any printer on any platform unchanged.
type Backend interface {
	// Print transmits already-rendered ESC/POS bytes to the device.
	//
	// Implementations must honour ctx cancellation, and must wrap failures with
	// the sentinel errors in this package (ErrDisconnected, ErrPaperOut) so
	// callers can branch with errors.Is rather than by matching on message text.
	Print(ctx context.Context, data []byte) error

	// Status probes device reachability and paper state without printing.
	Status(ctx context.Context) (Status, error)

	// Name identifies the backend/device for logs and telemetry.
	Name() string
}
