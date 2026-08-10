// Package device is the vocabulary every other package in this library shares:
// what a printer is, what a driver for one must be able to do, and how one is
// discovered.
//
// It depends on nothing but the standard library. That is deliberate — it lets
// the transport packages and the root package both refer to these types without
// importing each other, which is what keeps the dependency graph acyclic.
package device

import (
	"fmt"
	"strings"
)

// Connection is how a printer is attached. It drives nothing technical — the
// backend already knows how to talk to the device — but an operator choosing
// between two printers needs to know which one is the box on the counter and
// which one is across the shop.
type Connection string

const (
	ConnectionUSB     Connection = "usb"
	ConnectionNetwork Connection = "network"
)

// Device describes one printer the library can see, in terms a settings screen
// can render and an operator can recognise.
type Device struct {
	// ID identifies this printer across restarts, so a chosen printer is still
	// the chosen printer tomorrow. Shaped "<transport>:<what makes it unique>":
	//
	//   windows:EPSON TM-T82X-II      a Windows print queue
	//   cups:TM-T82X-II               a CUPS queue (Linux/macOS)
	//   usb:/dev/usb/lp0              a USB device node (Linux)
	//   net:192.168.1.50:9100         a printer with its own address
	//
	// A serial number is preferred over a port wherever one is available: move
	// a USB printer to a different socket and the serial follows it.
	ID string `json:"id"`

	// Name is what the operator sees. The queue name, or the host for a
	// network printer.
	Name string `json:"name"`

	Connection Connection `json:"connection"`

	// Transport names the mechanism behind it ("windows-spooler", "cups",
	// "tcp"). Diagnostic detail — two printers can share a connection kind and
	// be reached completely differently.
	Transport string `json:"transport"`

	// Receipt marks a printer recognised as a thermal/receipt printer rather
	// than an office printer or a virtual one. False does not mean unusable —
	// it means "we could not tell", and the operator can still pick it.
	Receipt bool `json:"receipt"`

	// Detail is free text for a settings screen: a model, an address, or why
	// something looks unusual.
	Detail string `json:"detail,omitempty"`
}

// NetworkDeviceID builds the stable ID for a printer reached over TCP.
func NetworkDeviceID(host string, port int) string {
	return fmt.Sprintf("net:%s:%d", host, port)
}

// QueueDeviceID builds the stable ID for a printer reached through an operating
// system print queue. transport is "windows" or "cups".
func QueueDeviceID(transport, queueName string) string {
	return transport + ":" + strings.TrimSpace(queueName)
}
