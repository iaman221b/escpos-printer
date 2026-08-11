package escposprinter

import "github.com/iaman221b/escpos-printer/device"

// Re-exports of the device package, so a caller doing ordinary work never needs
// a second import.
//
// These are aliases, not distinct types: device.Device and
// escposprinter.Device are the same type, and a value of one satisfies the
// other without conversion. Implementing a custom Backend or Finder against
// either name works identically.

type (
	// Device describes one printer, in terms a settings screen can render and
	// an operator can recognise.
	Device = device.Device

	// Discovered pairs a described device with a backend ready to print to it.
	Discovered = device.Discovered

	// Status is the result of a printer health probe.
	Status = device.Status

	// PaperStatus is what a probe could learn about the paper roll.
	PaperStatus = device.PaperStatus

	// Backend is one concrete way of talking to one printer. Implement it to
	// add a transport this library does not ship.
	Backend = device.Backend

	// Finder answers "what printers can you see right now?" for one mechanism.
	// Implement it to add a discovery source, and pass it to WithExtraFinders.
	Finder = device.Finder

	// Connection is how a printer is attached.
	Connection = device.Connection
)

// Paper states.
const (
	PaperOK  = device.PaperOK
	PaperLow = device.PaperLow
	PaperOut = device.PaperOut
)

// Connection kinds.
const (
	ConnectionUSB     = device.ConnectionUSB
	ConnectionNetwork = device.ConnectionNetwork
)
