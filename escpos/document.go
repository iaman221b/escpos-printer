package escpos

import "strings"

// A Document is a receipt described as data rather than as a sequence of
// method calls.
//
// The point is that a *style* — a sale receipt, a kitchen ticket, a gift
// receipt — becomes an ordinary function from your business data to a
// Document. Adding a style is adding a function; it needs nothing new from this
// package. And because a Document is data, it can be rendered more than one
// way: Render produces printer bytes, RenderText produces something readable
// that a golden test can diff.
//
//	func StandardReceipt(order Order) escpos.Document {
//	    return escpos.Document{
//	        escpos.Text{Value: order.Store, Bold: true, Align: escpos.AlignCenter},
//	        escpos.Rule{},
//	        escpos.LeftRight("TOTAL", "13.65"),
//	        escpos.QR{Data: "https://example.com/orders/1043"},
//	        escpos.Cut{},
//	    }
//	}
type Document []Element

// Element is one item in a Document. The set is closed: implementations live
// in this package so that every renderer can handle all of them.
type Element interface{ element() }

// TextSize scales characters. Width and Height are multipliers from 1 to 8;
// zero means 1. Useful for a kitchen ticket, where the item name has to be
// readable across a counter.
type TextSize struct {
	Width  uint8
	Height uint8
}

func (s TextSize) normalize() TextSize {
	if s.Width == 0 {
		s.Width = 1
	}
	if s.Height == 0 {
		s.Height = 1
	}
	if s.Width > 8 {
		s.Width = 8
	}
	if s.Height > 8 {
		s.Height = 8
	}
	return s
}

func (s TextSize) isDefault() bool {
	n := s.normalize()
	return n.Width == 1 && n.Height == 1
}

// Text is a line of characters followed by a line feed.
type Text struct {
	Value     string
	Bold      bool
	Underline bool
	Align     Alignment
	Size      TextSize
}

// Rule is a full-width horizontal separator.
type Rule struct {
	// Char is the character to repeat. Zero means '-'.
	Char rune
}

// Cell is one column of a Row.
type Cell struct {
	Text string
	// Width is the column width in characters. Zero means flexible: the cell
	// shares whatever space the fixed-width cells leave.
	Width int
	Align Alignment
}

// Row is a line of aligned columns, laid out against the profile's width.
//
// This is what keeps a style independent of the paper size. Give the variable
// part a zero Width and pin the columns that must not move:
//
//	escpos.Row{Cells: []escpos.Cell{
//	    {Text: item.Name},                                  // flexible
//	    {Text: qty,   Width: 4, Align: escpos.AlignRight},
//	    {Text: total, Width: 7, Align: escpos.AlignRight},
//	}}
//
// The same Row renders correctly at 32 and 48 columns without the style
// knowing which it is printing on.
type Row struct {
	Cells []Cell

	// Bold and Underline emphasise the whole line. A totals row is the usual
	// reason: it needs the column alignment a Row gives and the emphasis a
	// Text gives, and without these it would have to be built as a preformatted
	// string — which would hardcode the paper width back into the style.
	Bold      bool
	Underline bool
}

// LeftRight is the two-column row that makes up most of a receipt's totals
// block: a label on the left, a figure right-aligned against the far edge.
func LeftRight(left, right string) Row {
	return Row{Cells: []Cell{
		{Text: left},
		{Text: right, Width: len(right), Align: AlignRight},
	}}
}

// QR is a QR code. The zero value of every field except Data is valid and
// means the defaults — Model 2, module size 6, correction M.
type QR struct {
	Data       string
	Model      QRModel
	ModuleSize uint8
	Correction QRErrorCorrection
	Align      Alignment
}

// Barcode is a linear barcode. The zero value of every field except Data means
// Code128, height 162, width 3, human-readable text below.
type Barcode struct {
	Data   string
	Format BarcodeFormat
	Height uint8
	Width  uint8
	HRI    HRIPosition
	Align  Alignment
}

// Feed advances the paper.
type Feed struct {
	Lines int
}

// Cut cuts the paper, feeding the printed area clear of the cutter first —
// see DefaultCutFeed for why that is not optional. The distance comes from the
// Profile, so a printer with a different head-to-cutter gap needs no change to
// the style.
type Cut struct {
	Partial bool
}

func (Text) element()    {}
func (Rule) element()    {}
func (Row) element()     {}
func (QR) element()      {}
func (Barcode) element() {}
func (Feed) element()    {}
func (Cut) element()     {}

// layout renders a Row to a single line of the given width.
//
// Fixed-width cells keep their width; the remainder is shared between the
// flexible ones. Cells are separated by one space. Text that does not fit its
// column is truncated rather than wrapped — a receipt line that reflows would
// break the alignment of every column after it.
func (r Row) layout(columns int) string {
	if len(r.Cells) == 0 {
		return ""
	}

	separators := len(r.Cells) - 1
	available := columns - separators
	if available < len(r.Cells) {
		// Degenerate: not even one character per column. Give each cell one.
		available = len(r.Cells)
	}

	fixed, flexCount := 0, 0
	for _, c := range r.Cells {
		if c.Width > 0 {
			fixed += c.Width
		} else {
			flexCount++
		}
	}

	// Each flexible cell gets an equal share of what is left, with any
	// remainder going to the first — so a two-column row on an odd width does
	// not lose a character.
	flexEach, flexExtra := 0, 0
	if flexCount > 0 {
		remaining := available - fixed
		if remaining < flexCount {
			remaining = flexCount // at least one character each
		}
		flexEach = remaining / flexCount
		flexExtra = remaining % flexCount
	}

	var b strings.Builder
	for i, c := range r.Cells {
		width := c.Width
		if width == 0 {
			width = flexEach
			if flexExtra > 0 {
				width++
				flexExtra--
			}
		}

		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(fitCell(c.Text, width, c.Align))
	}

	return b.String()
}

// fitCell truncates or pads text to exactly width characters.
func fitCell(text string, width int, align Alignment) string {
	if width <= 0 {
		return ""
	}
	if len(text) > width {
		return text[:width]
	}

	pad := width - len(text)
	switch align {
	case AlignRight:
		return strings.Repeat(" ", pad) + text
	case AlignCenter:
		left := pad / 2
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", pad-left)
	default:
		return text + strings.Repeat(" ", pad)
	}
}
