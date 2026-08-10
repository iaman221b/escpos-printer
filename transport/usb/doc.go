// Package usb writes ESC/POS bytes straight to a printer's device node.
//
// This is the fallback for the case CUPS cannot serve: cheap thermal printers
// frequently ship no CUPS driver at all, so they never become a queue, but the
// kernel's usblp driver still exposes them as a writable device. Since ESC/POS
// is just bytes, writing to the node is all that is needed.
//
// Platform notes:
//
//   - Linux is the primary target. /dev/usb/lp* and /dev/usblp* are the usblp
//     printer-class nodes; /dev/ttyUSB* covers models that present as a serial
//     adapter instead.
//   - macOS has no usblp driver, so a USB printer there is reachable only
//     through CUPS. The /dev/cu.* patterns here catch serial-attached printers
//     only.
//   - Windows is deliberately unsupported. Talking to USB directly there means
//     replacing the printer's driver with a generic one, which would break the
//     working spooler path — and is unnecessary, because a USB thermal printer
//     on Windows already appears as an ordinary print queue.
//
// The implementation is behind a "!windows" build tag; this file carries no tag
// so the package still exists when compiling for Windows.
package usb
