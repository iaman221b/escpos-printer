// Package windows reaches printers through the Windows print spooler.
//
// USB thermal printers appear as an ordinary spooler queue on a USB port (an
// Epson TM-series on USB001, for example), so there is no direct device handle
// to write to — the spooler is the supported path for USB on Windows, and it
// covers network queues too.
//
// The implementation is behind a "windows" build tag; this file carries no tag
// so that the package still exists (and `go build ./...` still succeeds) when
// compiling for Linux or macOS.
package windows
