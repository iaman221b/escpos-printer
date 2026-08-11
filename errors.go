package escposprinter

import "github.com/iaman221b/escpos-printer/device"

// The sentinel errors this library wraps. They are the same values as the ones
// in the device package, re-exported here so an application using only the
// top-level package never has to import device to handle an error.
//
//	if errors.Is(err, escposprinter.ErrPaperOut) { ... }
//
// Branching on these rather than on message text is the point: message matching
// works only while one team owns both sides of a call, and breaks the first time
// a message is reworded or a platform reports the same condition differently.
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
