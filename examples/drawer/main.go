// Command drawer opens the cash drawer wired into the selected printer.
//
//	go run ./examples/drawer
//
// A cash drawer is not a peripheral in its own right: it is an RJ-11 cable from
// the drawer into the printer's DK port. "Open the drawer" means "send five
// bytes to the printer", which is why this goes through a Printer handle.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	escposprinter "github.com/iaman221b/escpos-printer"
	"github.com/iaman221b/escpos-printer/escpos"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reg := escposprinter.New()
	if _, err := reg.Discover(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "some discovery sources failed: %v\n", err)
	}

	p, err := reg.Active()
	if err != nil {
		fmt.Fprintf(os.Stderr, "no printer to kick the drawer from: %v\n", err)
		os.Exit(1)
	}

	pulse := escpos.DefaultPulse()
	if err := p.OpenDrawer(ctx, pulse); err != nil {
		// Always a printer-link failure. The DK port has no read-back, so
		// nothing here can observe the drawer's mechanism.
		if errors.Is(err, escposprinter.ErrDisconnected) {
			fmt.Fprintf(os.Stderr, "could not reach the printer: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "pulse delivery failed: %v\n", err)
		}
		os.Exit(1)
	}

	// Deliberately not "the drawer opened" — only that the pulse was delivered.
	fmt.Printf("pulse delivered to %s (pin %d, %dms/%dms)\n",
		p.ID(), pulse.Pin, pulse.OnMs, pulse.OffMs)
}
