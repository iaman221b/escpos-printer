// Command print discovers a printer and prints a sample receipt.
//
//	go run ./examples/print
//	go run ./examples/print "EPSON TM-T82X-II"    # pin a specific queue
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

	var opts []escposprinter.Option
	if len(os.Args) > 1 {
		// A pinned printer outranks the automatic pick. Note the application
		// reads its own configuration and passes the value in — the library
		// never touches the environment itself.
		opts = append(opts, escposprinter.WithPin(os.Args[1]))
	}

	reg := escposprinter.New(opts...)
	if _, err := reg.Discover(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "some discovery sources failed: %v\n", err)
	}

	p, err := reg.Active()
	if err != nil {
		if errors.Is(err, escposprinter.ErrNoPrinter) {
			fmt.Fprintln(os.Stderr, "no printer selected — run ./examples/list to see what is available")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "could not get a printer: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("printing to %s\n", p.ID())

	// Check before printing rather than after a failed receipt.
	if st, err := p.Status(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "status probe: %v\n", err)
	} else {
		fmt.Printf("online=%v paper=%s\n", st.Online, st.Paper)
	}

	if err := p.Print(ctx, sampleReceipt()); err != nil {
		switch {
		case errors.Is(err, escposprinter.ErrPaperOut):
			fmt.Fprintln(os.Stderr, "out of paper")
		case errors.Is(err, escposprinter.ErrCoverOpen):
			fmt.Fprintln(os.Stderr, "close the printer cover")
		case errors.Is(err, escposprinter.ErrDisconnected):
			fmt.Fprintf(os.Stderr, "printer unreachable: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "print failed: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Println("printed")
}

// sampleReceipt shows the Builder in use. The layout is the application's
// business decision — the library has no opinion about what a receipt contains.
func sampleReceipt() []byte {
	const width = escpos.Width58mm

	b := escpos.NewBuilder()

	// The drawer kick leads the stream so the drawer opens at the earliest
	// possible instant, in the same job as the receipt.
	b.DrawerKick(escpos.DefaultPulse())

	b.Init().
		Align(escpos.AlignCenter).
		Bold(true).Line("THE CORNER SHOP").Bold(false).
		Line("Order #1043").
		Line(time.Now().Format("02 Jan 2026 15:04")).
		Rule(width).
		Align(escpos.AlignLeft)

	items := []struct {
		name  string
		qty   int
		total float64
	}{
		{"Flat white", 2, 7.00},
		{"Almond croissant", 1, 3.80},
		{"Sparkling water", 1, 2.20},
	}
	for _, it := range items {
		b.Line(fmt.Sprintf("%-20s x%-3d %6.2f", it.name, it.qty, it.total))
	}

	b.Rule(width).
		Line(fmt.Sprintf("%-24s %7.2f", "Subtotal", 13.00)).
		Line(fmt.Sprintf("%-24s %7.2f", "Tax", 0.65)).
		Bold(true).Line(fmt.Sprintf("%-24s %7.2f", "TOTAL", 13.65)).Bold(false).
		Line("Paid via: Card").
		Feed(1).
		Align(escpos.AlignCenter).
		Line("Thank you!")

	// Cut feeds the printed area past the cutter first — see escpos.DefaultCutFeed.
	b.Cut()

	return b.Bytes()
}
