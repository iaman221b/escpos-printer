//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/iaman221b/escpos-printer/device"
)

// SpoolerBackend prints by submitting a RAW job to a Windows print queue.
//
// The job is submitted with the "RAW" datatype, which makes the spooler hand
// the bytes to the device untouched. Without it the driver would render the
// ESC/POS control bytes as literal text and print a page of garbage.
//
// The Win32 surface this is built on — the procedure handles, the structures,
// the status bits, and the queue queries the finder also uses — lives in
// winspool.go.
type SpoolerBackend struct {
	// QueueName must match the queue exactly as it appears in Windows
	// "Printers & scanners".
	QueueName string
}

// NewSpoolerBackend returns a backend for the named Windows print queue.
func NewSpoolerBackend(queueName string) *SpoolerBackend {
	return &SpoolerBackend{QueueName: queueName}
}

func (w *SpoolerBackend) Name() string {
	return fmt.Sprintf("windows:%s", w.QueueName)
}

// openQueue opens the print queue, translating the common "queue is not there"
// failures into ErrDisconnected.
func (w *SpoolerBackend) openQueue() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(w.QueueName)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid queue name %q: %w", device.ErrDisconnected, w.QueueName, err)
	}

	var handle windows.Handle
	r1, _, lastErr := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&handle)),
		0,
	)
	if r1 == 0 {
		return 0, fmt.Errorf(
			"%w: could not open print queue %q: %v (installed queues: %v)",
			device.ErrDisconnected, w.QueueName, lastErr, ListQueues(),
		)
	}
	return handle, nil
}

// Print submits the already-rendered ESC/POS bytes as a single RAW job.
func (w *SpoolerBackend) Print(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("refusing to print an empty receipt")
	}

	handle, err := w.openQueue()
	if err != nil {
		return err
	}
	defer procClosePrinter.Call(uintptr(handle))

	docName, _ := windows.UTF16PtrFromString("ESC/POS receipt")
	datatype, _ := windows.UTF16PtrFromString("RAW")
	info := docInfo1{DocName: docName, Datatype: datatype}

	jobID, _, lastErr := procStartDocPrinterW.Call(
		uintptr(handle),
		1,
		uintptr(unsafe.Pointer(&info)),
	)
	if jobID == 0 {
		return fmt.Errorf("%w: spooler rejected the print job on %q: %v", device.ErrDisconnected, w.QueueName, lastErr)
	}

	// StartPagePrinter is advisory for RAW jobs but keeps the spooler's job
	// accounting straight, so its failure is not worth aborting the receipt
	// over.
	pageStarted, _, _ := procStartPagePrinter.Call(uintptr(handle))

	writeErr := w.writeAll(handle, data)

	if pageStarted != 0 {
		procEndPagePrinter.Call(uintptr(handle))
	}
	procEndDocPrinter.Call(uintptr(handle))

	return writeErr
}

// writeAll loops until every byte is accepted; WritePrinter is permitted to
// accept a short count, and a truncated ESC/POS stream would print a partial
// receipt with no cut.
func (w *SpoolerBackend) writeAll(handle windows.Handle, data []byte) error {
	total := 0
	for total < len(data) {
		chunk := data[total:]
		var written uint32

		ok, _, lastErr := procWritePrinter.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&chunk[0])),
			uintptr(len(chunk)),
			uintptr(unsafe.Pointer(&written)),
		)
		if ok == 0 {
			// Only a genuine out-of-paper errno maps to ErrPaperOut; anything
			// else keeps the conservative ErrDisconnected default.
			if errors.Is(lastErr, windows.ERROR_OUT_OF_PAPER) {
				return fmt.Errorf("%w: on %q: %v", device.ErrPaperOut, w.QueueName, lastErr)
			}
			return fmt.Errorf("%w: write failed on %q after %d of %d bytes: %v",
				device.ErrDisconnected, w.QueueName, total, len(data), lastErr)
		}
		if written == 0 {
			return fmt.Errorf("%w: spooler stopped accepting data on %q after %d of %d bytes",
				device.ErrDisconnected, w.QueueName, total, len(data))
		}
		total += int(written)
	}
	return nil
}

// Status probes the queue without printing, mapping the spooler's status bits
// onto the Online/Paper fields.
func (w *SpoolerBackend) Status(ctx context.Context) (device.Status, error) {
	handle, err := w.openQueue()
	if err != nil {
		return device.Status{Online: false, Paper: device.PaperOut}, err
	}
	defer procClosePrinter.Call(uintptr(handle))

	// Two-call pattern: ask for the required buffer size, then fill it.
	var needed uint32
	procGetPrinterW.Call(uintptr(handle), 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return device.Status{Online: false, Paper: device.PaperOut},
			fmt.Errorf("%w: could not size printer info for %q", device.ErrDisconnected, w.QueueName)
	}

	buf := make([]byte, needed)
	ok, _, lastErr := procGetPrinterW.Call(
		uintptr(handle),
		2,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
	)
	if ok == 0 {
		return device.Status{Online: false, Paper: device.PaperOut},
			fmt.Errorf("%w: could not read printer status for %q: %v", device.ErrDisconnected, w.QueueName, lastErr)
	}

	info := (*printerInfo2)(unsafe.Pointer(&buf[0]))

	online, paper, reason := classifyQueueState(info.Status, info.Attributes)
	if !online {
		// Returned as an error, not a silent false, so the reason reaches the
		// caller's diagnostics. "Disconnected" with no explanation is the kind
		// of thing that has someone checking cables for an hour when the real
		// answer was "someone ticked Use Printer Offline".
		return device.Status{Online: false, Paper: paper},
			fmt.Errorf("%w: printer %q is %s", sentinelFor(paper, reason), w.QueueName, reason)
	}

	return device.Status{Online: true, Paper: paper}, nil
}

// sentinelFor picks the sentinel that best describes an unhealthy queue, so
// callers can branch with errors.Is instead of reading the reason text.
func sentinelFor(paper device.PaperStatus, reason string) error {
	switch {
	case paper == device.PaperOut:
		return device.ErrPaperOut
	case reason == coverOpenReason:
		return device.ErrCoverOpen
	default:
		return device.ErrDisconnected
	}
}
