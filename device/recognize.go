package device

import "strings"

// receiptVendors are the manufacturers whose names on a print queue mean
// "thermal receipt printer" with reasonable confidence. Matched case
// insensitively against the queue/device name.
//
// Getting this right matters in both directions: it drives the automatic pick
// when exactly one receipt printer is present, and it keeps an office laser
// from being chosen for receipts.
var receiptVendors = []string{
	"epson", "star ", "starmicronics", "bixolon", "citizen", "snbc",
	"rongta", "xprinter", "posiflex", "sewoo", "custom ", "zebra",
	"tm-t", "tm-m", "tsp1", "tsp6", "tsp7", "srp-",
}

// virtualQueues are printers the operating system offers that are not devices
// at all. Choosing one silently swallows every receipt into a file dialog or a
// document, which looks exactly like a broken printer — so they are never
// auto-selected and are marked in the list.
var virtualQueues = []string{
	"print to pdf", "microsoft xps", "xps document writer", "onenote",
	"fax", "adobe pdf", "pdfcreator", "cutepdf", "send to onenote",
	"preview", "pdf",
}

// LooksLikeReceiptPrinter reports whether a queue/device name belongs to a
// known receipt printer. Virtual queues never qualify, whatever they are named.
func LooksLikeReceiptPrinter(name string) bool {
	if IsVirtualQueue(name) {
		return false
	}

	lowered := strings.ToLower(name)
	for _, vendor := range receiptVendors {
		if strings.Contains(lowered, vendor) {
			return true
		}
	}
	return false
}

// IsVirtualQueue reports whether a queue is one of the operating system's
// document-producing pseudo-printers rather than a physical device.
func IsVirtualQueue(name string) bool {
	lowered := strings.ToLower(name)
	for _, virtual := range virtualQueues {
		if strings.Contains(lowered, virtual) {
			return true
		}
	}
	return false
}

// RegisterReceiptVendor adds a name fragment that should be recognised as a
// thermal receipt printer, for hardware this library does not know about.
// Matching is case-insensitive and by substring.
func RegisterReceiptVendor(fragment string) {
	fragment = strings.ToLower(strings.TrimSpace(fragment))
	if fragment == "" {
		return
	}
	receiptVendors = append(receiptVendors, fragment)
}
