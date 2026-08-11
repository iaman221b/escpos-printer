package escpos

import (
	"errors"
	"fmt"
	"strings"
)

// Render turns a Document into the bytes for a printer matching the Profile.
//
// The bytes are always returned, even when an element could not be rendered:
// a barcode with an invalid payload costs you the barcode, not the receipt.
// The error reports what was dropped, and callers that print receipts for money
// should log it rather than fail the sale.
//
// Internally this drives a Builder, so there is exactly one place in this
// package that emits ESC/POS bytes.
func Render(doc Document, p Profile) ([]byte, error) {
	p = p.normalize()
	b := NewBuilder().Init()

	var problems []error

	for _, element := range doc {
		switch e := element.(type) {
		case Text:
			renderText(b, e)

		case Rule:
			b.Align(AlignLeft).Rule(p.Columns)

		case Row:
			b.Align(AlignLeft)
			if e.Bold {
				b.Bold(true)
			}
			if e.Underline {
				b.Underline(true)
			}
			b.Line(e.layout(p.Columns))
			if e.Underline {
				b.Underline(false)
			}
			if e.Bold {
				b.Bold(false)
			}

		case Feed:
			b.Feed(e.Lines)

		case QR:
			if !p.Caps.QR {
				problems = append(problems, fmt.Errorf("profile %q cannot print QR codes", p.Name))
				continue
			}
			b.Align(e.Align).QRCodeWith(e.Data, QRConfig{
				Model:      e.Model,
				ModuleSize: e.ModuleSize,
				Correction: e.Correction,
			})
			b.Align(AlignLeft)

		case Barcode:
			if !p.Caps.Barcode {
				problems = append(problems, fmt.Errorf("profile %q cannot print barcodes", p.Name))
				continue
			}
			b.Align(e.Align).BarcodeWith(e.Data, BarcodeConfig{
				Format: e.Format,
				Height: e.Height,
				Width:  e.Width,
				HRI:    e.HRI,
			})
			b.Align(AlignLeft)

		case Cut:
			if e.Partial {
				b.Feed(p.CutFeed).write(escPartCut)
			} else {
				b.CutAfter(p.CutFeed)
			}

		default:
			problems = append(problems, fmt.Errorf("unknown element %T", element))
		}
	}

	if err := b.Err(); err != nil {
		problems = append(problems, err)
	}

	return b.Bytes(), errors.Join(problems...)
}

func renderText(b *Builder, t Text) {
	b.Align(t.Align)

	if !t.Size.isDefault() {
		size := t.Size.normalize()
		b.Size(size.Width, size.Height)
	}
	if t.Bold {
		b.Bold(true)
	}
	if t.Underline {
		b.Underline(true)
	}

	b.Line(t.Value)

	// Style is reset after the line rather than left set, so a Document reads
	// as a list of independent elements: a bold header cannot leak into the
	// item that follows it.
	if t.Underline {
		b.Underline(false)
	}
	if t.Bold {
		b.Bold(false)
	}
	if !t.Size.isDefault() {
		b.Size(1, 1)
	}
}

// RenderText renders a Document as plain text, at the Profile's width.
//
// This is the same document the printer would receive, minus the control bytes
// — which makes it the practical way to test a layout. A golden file of this
// output is reviewable in a diff, whereas an assertion over ESC/POS bytes tells
// a reader nothing about whether the receipt looks right.
//
// Symbols cannot be drawn in text, so a QR or barcode appears as a placeholder
// line naming its payload. The purpose is to verify structure and content, not
// to reproduce the artwork.
//
// One thing it does not model: Text with a Size width multiplier prints fewer
// characters per line than the profile's Columns, because each character is
// physically wider. The text rendering shows the content, not that narrowing.
func RenderText(doc Document, p Profile) string {
	p = p.normalize()

	var out strings.Builder

	// Emphasis markers are annotations, so they are not counted against the
	// roll width. Fitting the decorated string to Columns would truncate the
	// end of an emphasised line that fits perfectly well on paper — a preview
	// that hides the total is worse than no preview.
	writeLine := func(text string, align Alignment, allowance int) {
		out.WriteString(fitCell(text, p.Columns+allowance, align))
		out.WriteByte('\n')
	}

	emphasise := func(value string, bold, underline bool) (string, int) {
		allowance := 0
		if bold {
			value, allowance = "**"+value+"**", allowance+4
		}
		if underline {
			value, allowance = "_"+value+"_", allowance+2
		}
		return value, allowance
	}

	for _, element := range doc {
		switch e := element.(type) {
		case Text:
			value, allowance := emphasise(e.Value, e.Bold, e.Underline)
			writeLine(value, e.Align, allowance)

		case Rule:
			char := e.Char
			if char == 0 {
				char = '-'
			}
			out.WriteString(strings.Repeat(string(char), p.Columns))
			out.WriteByte('\n')

		case Row:
			value, _ := emphasise(strings.TrimRight(e.layout(p.Columns), " "), e.Bold, e.Underline)
			out.WriteString(value)
			out.WriteByte('\n')

		case Feed:
			for i := 0; i < e.Lines; i++ {
				out.WriteByte('\n')
			}

		case QR:
			// Not fitted to the width: the placeholder is an annotation, not
			// printed content, and truncating it would hide the payload — which
			// is the one thing a test of this output needs to see.
			writeAnnotation(&out, "[QR: "+e.Data+"]", e.Align, p.Columns)

		case Barcode:
			writeAnnotation(&out, "[BARCODE: "+e.Data+"]", e.Align, p.Columns)

		case Cut:
			kind := "CUT"
			if e.Partial {
				kind = "PARTIAL CUT"
			}
			out.WriteString(strings.Repeat("=", p.Columns))
			out.WriteByte('\n')
			writeLine("["+kind+"]", AlignCenter, 0)
		}
	}

	return out.String()
}

// writeAnnotation emits a placeholder line, centred or indented to match the
// element's alignment but never truncated.
func writeAnnotation(out *strings.Builder, text string, align Alignment, columns int) {
	if len(text) < columns {
		out.WriteString(fitCell(text, columns, align))
	} else {
		out.WriteString(text)
	}
	out.WriteByte('\n')
}
