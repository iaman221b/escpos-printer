//go:build !windows

package usb

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/iaman221b/escpos-printer/device"
)

// deviceGlobs are where the kernel exposes USB printers.
//
// The first two are the Linux usblp printer-class nodes — a device appearing
// there is a printer. The rest are serial patterns, covering both the receipt
// models that present as a USB-serial adapter instead of a printer class device
// and the macOS equivalents; those are offered but never flagged as receipt
// printers, since plenty of non-printers appear the same way.
var deviceGlobs = []string{
	"/dev/usb/lp*",       // Linux, printer class
	"/dev/usblp*",        // Linux, printer class
	"/dev/ttyUSB*",       // Linux, USB-serial
	"/dev/cu.usbserial*", // macOS, USB-serial
	"/dev/cu.usbmodem*",  // macOS, USB-modem/serial
}

// printerClassGlobs are the patterns whose matches are known printers rather
// than "some serial device that might be one".
var printerClassGlobs = []string{"/dev/usb/lp", "/dev/usblp"}

// Finder lists USB printers exposed as device nodes.
//
// It is designed to run alongside the CUPS finder rather than instead of it: a
// printer installed as a CUPS queue will appear twice, once per route, and the
// operator picks whichever works. That is better than guessing which route is
// correct for hardware that cannot be inspected.
type Finder struct{}

// NewFinder returns a USB device-node finder.
func NewFinder() *Finder { return &Finder{} }

func (f *Finder) Name() string { return "usb-device" }

func (f *Finder) Find(ctx context.Context) ([]device.Discovered, error) {
	var found []device.Discovered

	for _, pattern := range deviceGlobs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, path := range matches {
			isPrinterClass := isPrinterClassNode(path)

			detail := "USB printer device"
			if !isPrinterClass {
				detail = "USB serial device — may or may not be a printer"
			}

			found = append(found, device.Discovered{
				Device: device.Device{
					ID:         "usb:" + path,
					Name:       path,
					Connection: device.ConnectionUSB,
					Transport:  "usb-device",
					// A serial node is a guess, so it is offered but not
					// flagged as a receipt printer, which keeps it out of the
					// automatic pick.
					Receipt: isPrinterClass,
					Detail:  detail,
				},
				Backend: NewBackend(path),
			})
		}
	}

	return found, nil
}

func isPrinterClassNode(path string) bool {
	for _, prefix := range printerClassGlobs {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
