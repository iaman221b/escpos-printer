// Package escposprinter finds, selects, and prints to ESC/POS thermal receipt
// printers, and opens the cash drawer wired into one.
//
// It picks the right transport for the operating system at compile time: the
// print spooler on Windows, CUPS and USB device nodes on Linux and macOS, and
// raw TCP everywhere. Platform code that does not apply is not merely skipped —
// it is never compiled in, so a Linux build contains no Windows syscalls at all.
//
//	reg := escposprinter.New(
//	    escposprinter.WithStore(myDB),
//	    escposprinter.WithNetwork(network.Config{Hosts: []string{"192.168.1.50"}}),
//	)
//
//	devices, err := reg.Discover(ctx)
//	err = reg.Select(ctx, "windows:EPSON TM-T82X-II")
//
//	p, err := reg.Active()
//	err = p.Print(ctx, receiptBytes)
//	st, err := p.Status(ctx)
//	err = p.OpenDrawer(ctx, escpos.DefaultPulse())
//
// # What this library does not do
//
// It does not read environment variables, own a database, or define what a
// receipt contains. Those are the calling application's decisions:
//
//   - Configuration is passed in through Options; nothing is read from the host.
//   - Remembering the operator's chosen printer is done through the
//     SelectionStore interface, which the application implements over whatever
//     storage it already has.
//   - Receipt layout is built with the escpos package's Builder. Store names,
//     line items, and totals are business data this library has no opinion on.
//
// # Errors
//
// Failures wrap the sentinels in this package, so callers branch with
// errors.Is rather than by matching message text:
//
//	if errors.Is(err, escposprinter.ErrPaperOut) { ... }
package escposprinter
