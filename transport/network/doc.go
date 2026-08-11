// Package network reaches printers that have their own address, over a raw TCP
// socket on the ESC/POS port.
//
// Sockets are sockets: this package carries no build tag and uses nothing
// platform specific, so the same code finds and prints to LAN printers on
// Windows, Linux and macOS alike. It is the one transport in this library that
// is identical everywhere.
//
// Discovery is off unless the caller asks for it — see Config. Sweeping a shop
// or corporate network unprompted is rude and can look like a port scan.
package network
