package escposprinter

import "github.com/iaman221b/escpos-printer/device"

// The sentinel errors this library wraps. They are the same values as the ones
// in the device package, re-exported here so an application using only the
// top-level package never has to import device to handle an error.
//
//	if errors.Is(err, escposprinter.ErrPaperOut) { ... }
var (
	// ErrNoPrinter means no printer is available at all. It is an answer, not
	// a malfunction: a terminal with nothing plugged in still takes payments,
	// it just cannot produce paper.
	ErrNoPrinter = device.ErrNoPrinter

	// ErrUnknownPrinter means a request named a printer this registry does not
	// hold — usually one that has since been unplugged.
	ErrUnknownPrinter = device.ErrUnknownPrinter

	// ErrDisconnected means the device could not be reached.
	ErrDisconnected = device.ErrDisconnected

	// ErrPaperOut means the device reported that it is out of paper.
	ErrPaperOut = device.ErrPaperOut

	// ErrCoverOpen means the device reported its cover or door is open.
	ErrCoverOpen = device.ErrCoverOpen

	// ErrUnsupported means the requested transport does not exist on this
	// operating system.
	ErrUnsupported = device.ErrUnsupported
)

// Convenience aliases, so a caller doing ordinary work never needs a second
// import. These are aliases, not distinct types: device.Device and
// escposprinter.Device are the same type.
type (
	Device      = device.Device
	Discovered  = device.Discovered
	Status      = device.Status
	PaperStatus = device.PaperStatus
	Backend     = device.Backend
	Finder      = device.Finder
	Connection  = device.Connection
)

// Re-exported paper states.
const (
	PaperOK  = device.PaperOK
	PaperLow = device.PaperLow
	PaperOut = device.PaperOut
)

// Re-exported connection kinds.
const (
	ConnectionUSB     = device.ConnectionUSB
	ConnectionNetwork = device.ConnectionNetwork
)
