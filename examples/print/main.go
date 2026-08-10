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

	profile := escpos.Profile58mm
	doc := sampleReceipt()

	// The same document, readable — this is what a golden test would assert on.
	fmt.Printf("\n%s\n", escpos.RenderText(doc, profile))

	data, err := escpos.Render(doc, profile)
	if err != nil {
		// Bytes are still returned; the error names what was dropped. A receipt
		// missing its barcode is better than a sale that failed to print.
		fmt.Fprintf(os.Stderr, "some elements were dropped: %v\n", err)
	}

	if err := p.Print(ctx, data); err != nil {
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

// sampleReceipt shows the document model. The layout is the application's
// business decision — the library has no opinion about what a receipt contains.
func sampleReceipt() escpos.Document {
	items := []struct {
		name  string
		qty   int
		total float64
	}{
		{"Flat white", 2, 7.00},
		{"Almond croissant", 1, 3.80},
		{"Sparkling water", 1, 2.20},
	}

	doc := escpos.Document{
		escpos.Text{Value: "THE CORNER SHOP", Bold: true, Align: escpos.AlignCenter},
		escpos.Text{Value: "Order #ORD-1043", Align: escpos.AlignCenter},
		escpos.Text{Value: time.Now().Format("02 Jan 2006 15:04"), Align: escpos.AlignCenter},
		escpos.Rule{},
	}

	for _, it := range items {
		doc = append(doc, escpos.Row{Cells: []escpos.Cell{
			{Text: it.name},
			{Text: fmt.Sprintf("x%d", it.qty), Width: 4, Align: escpos.AlignRight},
			{Text: fmt.Sprintf("%.2f", it.total), Width: 7, Align: escpos.AlignRight},
		}})
	}

	return append(doc,
		escpos.Rule{},
		escpos.LeftRight("Subtotal", "13.00"),
		escpos.LeftRight("Tax", "0.65"),
		escpos.Text{Value: fmt.Sprintf("%-24s %7.2f", "TOTAL", 13.65), Bold: true},
		escpos.Text{Value: "Paid via: Card"},
		escpos.Feed{Lines: 1},
		escpos.Text{Value: "Thank you!", Align: escpos.AlignCenter},
		escpos.Feed{Lines: 1},
		// Firmware-rendered: the printer encodes both of these itself.
		escpos.Barcode{Data: "ORD-1043", Align: escpos.AlignCenter},
		escpos.Feed{Lines: 1},
		escpos.QR{Data: "https://example.com/orders/ORD-1043", Align: escpos.AlignCenter},
		escpos.Cut{},
	)
}
