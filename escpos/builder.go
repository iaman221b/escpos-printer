// Package escpos builds ESC/POS byte streams for thermal receipt printers.
//
// It produces bytes and nothing else — it never opens a socket, a queue, or a
// device node. That separation is the whole point: one rendered byte stream is
// delivered unchanged by any transport on any operating system, so a receipt
// printed from a Windows till is byte-identical to the same receipt printed
// from a Mac one.
//
// This package deliberately does not know what a receipt contains. Store names,
// line items, tax, and totals are the calling application's business contract,
// not this library's. Builder gives you the primitives; you decide the layout.
package escpos

import (
	"bytes"
	"strings"
)

// Roll widths in characters, for the two common thermal paper sizes. Passed to
// Rule and used by callers to size their own column layouts.
const (
	Width58mm = 32
	Width80mm = 48
)

// ESC/POS control sequences. Hand-rolled rather than pulled from a dependency:
// a receipt needs only init, emphasis, alignment, feed, and cut, and these five
// sequences have not changed since the 1990s.
const (
	escInit     = "\x1b\x40"     // ESC @   — initialize printer
	escBoldOn   = "\x1b\x45\x01" // ESC E 1 — bold on
	escBoldOff  = "\x1b\x45\x00" // ESC E 0 — bold off
	escAlignLft = "\x1b\x61\x00" // ESC a 0 — left align
	escAlignCtr = "\x1b\x61\x01" // ESC a 1 — center align
	escAlignRgt = "\x1b\x61\x02" // ESC a 2 — right align
	escUnderOn  = "\x1b\x2d\x01" // ESC - 1 — underline on
	escUnderOff = "\x1b\x2d\x00" // ESC - 0 — underline off
	escCut      = "\x1d\x56\x00" // GS V 0  — full cut
	escPartCut  = "\x1d\x56\x01" // GS V 1  — partial cut
	lf          = "\x0a"         // LF
)

// DefaultCutFeed is how many lines Cut feeds before cutting.
//
// This is not cosmetic spacing, it is what makes a receipt readable at all.
// The print head sits roughly 15mm above the cutter, so the last few
// centimetres of a receipt are still inside the mechanism at the moment the cut
// happens. Cutting without feeding first slices through blank leader paper and
// leaves the printed text stranded inside the printer — the slip that falls out
// is blank, and the text only emerges later, pushed out by the next job.
//
// Six lines clears the head-to-cutter gap on an Epson TM-T82X-II with margin.
// A printer with a different gap can override it with CutAfter.
//
// Cut applies this automatically so that a caller who has never taken a thermal
// printer apart cannot get it wrong.
const DefaultCutFeed = 6

// Alignment selects horizontal text alignment.
type Alignment uint8

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// Builder accumulates an ESC/POS byte stream. Methods chain; the zero value is
// ready to use, though NewBuilder reads better at a call site.
//
// A Builder is not safe for concurrent use.
type Builder struct {
	buf bytes.Buffer
	err error
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder { return &Builder{} }

// Err reports the first error recorded while building, or nil.
//
// The methods chain, so they cannot return errors individually. Instead the
// first failure is remembered and everything after it still runs — a bad
// barcode payload costs you the barcode, not the receipt. Check this once
// before dispatching if you care.
func (b *Builder) Err() error { return b.err }

// fail records the first error and emits nothing for the failed element.
func (b *Builder) fail(err error) *Builder {
	if b.err == nil {
		b.err = err
	}
	return b
}

// Init emits the printer reset sequence. Start every job with it: it clears
// whatever style state a previous job left behind.
func (b *Builder) Init() *Builder { return b.write(escInit) }

// Bold turns emphasis on or off.
func (b *Builder) Bold(on bool) *Builder {
	if on {
		return b.write(escBoldOn)
	}
	return b.write(escBoldOff)
}

// Underline turns underlining on or off.
func (b *Builder) Underline(on bool) *Builder {
	if on {
		return b.write(escUnderOn)
	}
	return b.write(escUnderOff)
}

// Align sets horizontal alignment for subsequent text.
func (b *Builder) Align(a Alignment) *Builder {
	switch a {
	case AlignCenter:
		return b.write(escAlignCtr)
	case AlignRight:
		return b.write(escAlignRgt)
	default:
		return b.write(escAlignLft)
	}
}

// Size scales characters. Width and Height are multipliers from 1 to 8; values
// outside that range are clamped. Size(1, 1) restores the default.
func (b *Builder) Size(width, height uint8) *Builder {
	clamp := func(v uint8) uint8 {
		if v < 1 {
			return 1
		}
		if v > 8 {
			return 8
		}
		return v
	}
	// GS ! n — the high nibble is the width multiplier, the low nibble the
	// height, each stored one less than the factor.
	n := (clamp(width)-1)<<4 | (clamp(height) - 1)
	return b.writeBytes([]byte{0x1d, 0x21, n})
}

// Text writes characters with no trailing line feed.
func (b *Builder) Text(s string) *Builder { return b.write(s) }

// Line writes characters followed by a line feed.
func (b *Builder) Line(s string) *Builder { return b.write(s).write(lf) }

// Feed advances the paper by n lines.
func (b *Builder) Feed(n int) *Builder {
	if n < 1 {
		return b
	}
	// ESC d n — print and feed n lines. One byte holds the count.
	if n > 255 {
		n = 255
	}
	return b.writeBytes([]byte{0x1b, 0x64, byte(n)})
}

// Rule writes a full-width horizontal line of dashes followed by a line feed.
func (b *Builder) Rule(width int) *Builder {
	if width < 1 {
		width = Width58mm
	}
	return b.Line(strings.Repeat("-", width))
}

// Raw appends bytes verbatim, for ESC/POS commands this package does not wrap.
func (b *Builder) Raw(p []byte) *Builder { return b.writeBytes(p) }

// DrawerKick appends the cash drawer pulse.
//
// Prepend it — call this before Init — when the drawer should open at the
// earliest possible instant of the job. The init sequence resets print state,
// not the DK port, so the kick still fires from wherever it sits in the stream.
func (b *Builder) DrawerKick(p DrawerPulse) *Builder {
	return b.writeBytes(p.Normalize().ToBytes())
}

// Cut feeds the printed area clear of the cutter and then cuts.
//
// See DefaultCutFeed for why the feed is not optional.
func (b *Builder) Cut() *Builder { return b.CutAfter(DefaultCutFeed) }

// CutAfter feeds the given number of lines and then cuts, for printers whose
// head-to-cutter gap differs from the Epson default.
func (b *Builder) CutAfter(lines int) *Builder {
	return b.Feed(lines).write(escCut)
}

// PartialCut feeds and then cuts, leaving a small tab of paper uncut so the
// receipt stays attached until it is torn off.
func (b *Builder) PartialCut() *Builder {
	return b.Feed(DefaultCutFeed).write(escPartCut)
}

// Bytes returns the accumulated stream. The Builder may be used further
// afterwards; the returned slice is a copy.
func (b *Builder) Bytes() []byte {
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

// Len reports how many bytes have been accumulated.
func (b *Builder) Len() int { return b.buf.Len() }

// Reset empties the Builder for reuse, clearing any recorded error.
func (b *Builder) Reset() *Builder {
	b.buf.Reset()
	b.err = nil
	return b
}

func (b *Builder) write(s string) *Builder {
	b.buf.WriteString(s)
	return b
}

func (b *Builder) writeBytes(p []byte) *Builder {
	b.buf.Write(p)
	return b
}
