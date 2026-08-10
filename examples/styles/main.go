// Command styles shows the point of the document model: several receipt styles
// over the same data, each rendered at more than one paper width, with no
// printer involved.
//
//	go run ./examples/styles
//
// A style is an ordinary function returning an escpos.Document. Adding one
// needs nothing from the library.
package main

import (
	"fmt"
	"strings"

	"github.com/iaman221b/escpos-printer/escpos"
)

// Order is the caller's own business data. The library never sees this type.
type Order struct {
	Store  string
	Number string
	Items  []Item
	Total  float64
	Paid   string
}

type Item struct {
	Name  string
	Qty   int
	Total float64
}

func main() {
	order := Order{
		Store:  "THE CORNER SHOP",
		Number: "ORD-1043",
		Items: []Item{
			{"Flat white", 2, 7.00},
			{"Almond croissant with butter", 1, 3.80},
			{"Sparkling water", 1, 2.20},
		},
		Total: 13.00,
		Paid:  "Card",
	}

	styles := []struct {
		name string
		doc  escpos.Document
	}{
		{"customer receipt", StandardReceipt(order)},
		{"kitchen ticket", KitchenTicket(order)},
		{"gift receipt", GiftReceipt(order)},
	}

	for _, profile := range []escpos.Profile{escpos.Profile58mm, escpos.Profile80mm} {
		for _, style := range styles {
			fmt.Printf("\n=== %s @ %s (%d columns) ===\n\n",
				style.name, profile.Name, profile.Columns)
			fmt.Print(escpos.RenderText(style.doc, profile))

			// The same document also renders to printer bytes.
			data, err := escpos.Render(style.doc, profile)
			if err != nil {
				fmt.Printf("\n(dropped elements: %v)\n", err)
			}
			fmt.Printf("\n%d bytes for the printer\n", len(data))
		}
	}
}

// StandardReceipt is what the customer takes away.
func StandardReceipt(o Order) escpos.Document {
	doc := escpos.Document{
		escpos.Text{Value: o.Store, Bold: true, Align: escpos.AlignCenter},
		escpos.Text{Value: "Order #" + o.Number, Align: escpos.AlignCenter},
		escpos.Rule{},
	}

	for _, it := range o.Items {
		doc = append(doc, escpos.Row{Cells: []escpos.Cell{
			{Text: it.Name},
			{Text: fmt.Sprintf("x%d", it.Qty), Width: 4, Align: escpos.AlignRight},
			{Text: fmt.Sprintf("%.2f", it.Total), Width: 7, Align: escpos.AlignRight},
		}})
	}

	return append(doc,
		escpos.Rule{},
		escpos.LeftRight("TOTAL", fmt.Sprintf("%.2f", o.Total)),
		escpos.Text{Value: "Paid via: " + o.Paid},
		escpos.Feed{Lines: 1},
		escpos.QR{Data: "https://example.com/orders/" + o.Number, Align: escpos.AlignCenter},
		escpos.Cut{},
	)
}

// KitchenTicket is read across a counter, so it is large text and no money.
func KitchenTicket(o Order) escpos.Document {
	doc := escpos.Document{
		escpos.Text{
			Value: "#" + strings.TrimPrefix(o.Number, "ORD-"),
			Bold:  true,
			Align: escpos.AlignCenter,
			Size:  escpos.TextSize{Width: 2, Height: 2},
		},
		escpos.Rule{Char: '='},
	}

	for _, it := range o.Items {
		doc = append(doc, escpos.Text{
			Value: fmt.Sprintf("%d x %s", it.Qty, it.Name),
			Size:  escpos.TextSize{Height: 2},
		})
	}

	return append(doc, escpos.Feed{Lines: 1}, escpos.Cut{})
}

// GiftReceipt proves the order without revealing what was paid.
func GiftReceipt(o Order) escpos.Document {
	doc := escpos.Document{
		escpos.Text{Value: o.Store, Bold: true, Align: escpos.AlignCenter},
		escpos.Text{Value: "Gift receipt", Align: escpos.AlignCenter},
		escpos.Rule{},
	}

	for _, it := range o.Items {
		doc = append(doc, escpos.Row{Cells: []escpos.Cell{
			{Text: it.Name},
			{Text: fmt.Sprintf("x%d", it.Qty), Width: 4, Align: escpos.AlignRight},
		}})
	}

	return append(doc,
		escpos.Rule{},
		escpos.Text{Value: "Exchangeable within 30 days", Align: escpos.AlignCenter},
		escpos.Feed{Lines: 1},
		escpos.Barcode{Data: o.Number, Align: escpos.AlignCenter},
		escpos.Cut{},
	)
}
