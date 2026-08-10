package device

import "errors"

// The sentinel errors every backend and finder in this library wraps.
//
// These exist so callers can branch on what went wrong with errors.Is, rather
// than by searching the error's message text for a keyword. Message matching
// works only while one team owns both sides of the call; across a library
// boundary it breaks the first time a message is reworded or a platform
// reports the same condition in different English.
var (
	// ErrNoPrinter means no printer is available at all. It is an answer, not
	// a malfunction: a terminal with nothing plugged in still takes payments,
	// it just cannot produce paper. Callers report it and carry on.
	ErrNoPrinter = errors.New("no printer is available")

	// ErrUnknownPrinter means a request named a printer this library does not
	// have — usually one that has since been unplugged.
	ErrUnknownPrinter = errors.New("unknown printer")

	// ErrDisconnected means the device could not be reached: the queue is gone,
	// the socket refused, the device node vanished, the write failed partway.
	// It is the conservative default for any transport failure that is not
	// specifically a paper condition.
	ErrDisconnected = errors.New("printer disconnected")

	// ErrPaperOut means the device reported that it is out of paper. Only
	// returned where a transport can genuinely observe it — a Windows spooler
	// queue can, a CUPS queue and a USB device node cannot.
	ErrPaperOut = errors.New("printer is out of paper")

	// ErrCoverOpen means the device reported its cover or door is open.
	ErrCoverOpen = errors.New("printer cover is open")

	// ErrUnsupported means the requested transport does not exist on this
	// operating system — asking for the Windows spooler on Linux, for example.
	ErrUnsupported = errors.New("transport is not supported on this platform")
)
