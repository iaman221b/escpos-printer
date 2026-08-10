package escpos

import (
	"bytes"
	"testing"
)

// The regression this whole file exists for. Cutting without feeding first
// slices through blank leader paper and leaves the printed text stranded
// inside the mechanism — the slip that falls out is blank. Cut must always
// feed before it cuts.
func TestCutFeedsBeforeCutting(t *testing.T) {
	got := NewBuilder().Init().Line("TOTAL 25.00").Cut().Bytes()

	feed := []byte{0x1b, 0x64, DefaultCutFeed} // ESC d 6
	cut := []byte{0x1d, 0x56, 0x00}            // GS V 0

	feedAt := bytes.Index(got, feed)
	cutAt := bytes.Index(got, cut)

	if feedAt < 0 {
		t.Fatalf("no ESC d feed before the cut: % x", got)
	}
	if cutAt < 0 {
		t.Fatalf("no GS V cut emitted: % x", got)
	}
	if feedAt > cutAt {
		t.Fatalf("feed at %d comes after cut at %d; the receipt would come out blank", feedAt, cutAt)
	}
}

func TestPartialCutAlsoFeedsFirst(t *testing.T) {
	got := NewBuilder().Line("x").PartialCut().Bytes()

	feedAt := bytes.Index(got, []byte{0x1b, 0x64, DefaultCutFeed})
	cutAt := bytes.Index(got, []byte{0x1d, 0x56, 0x01})

	if feedAt < 0 || cutAt < 0 || feedAt > cutAt {
		t.Fatalf("partial cut must feed before cutting: % x", got)
	}
}

func TestCutAfterHonoursCustomFeed(t *testing.T) {
	got := NewBuilder().CutAfter(12).Bytes()
	if bytes.Index(got, []byte{0x1b, 0x64, 12}) < 0 {
		t.Fatalf("CutAfter(12) did not emit ESC d 12: % x", got)
	}
}

func TestInitIsEmittedFirst(t *testing.T) {
	got := NewBuilder().Init().Text("hello").Bytes()
	if !bytes.HasPrefix(got, []byte{0x1b, 0x40}) {
		t.Fatalf("stream does not start with ESC @: % x", got)
	}
}

// The drawer kick is prepended to the same byte stream as the receipt rather
// than sent as its own job. A printer processes one job at a time, so this
// makes "drawer opens before the receipt prints" a property of the byte order
// itself — no second job can be scheduled in between.
func TestDrawerKickCanPrecedeInit(t *testing.T) {
	got := NewBuilder().DrawerKick(DefaultPulse()).Init().Line("receipt").Cut().Bytes()

	kickAt := bytes.Index(got, []byte{0x1b, 0x70})
	initAt := bytes.Index(got, []byte{0x1b, 0x40})

	if kickAt != 0 {
		t.Fatalf("drawer kick should lead the stream, found at %d: % x", kickAt, got)
	}
	if kickAt > initAt {
		t.Fatalf("kick at %d after init at %d", kickAt, initAt)
	}
}

func TestAlignmentAndBoldEmitExpectedCodes(t *testing.T) {
	got := NewBuilder().Align(AlignCenter).Bold(true).Text("HDR").Bold(false).Align(AlignLeft).Bytes()

	for _, want := range [][]byte{
		{0x1b, 0x61, 0x01}, // center
		{0x1b, 0x45, 0x01}, // bold on
		{0x1b, 0x45, 0x00}, // bold off
		{0x1b, 0x61, 0x00}, // left
	} {
		if !bytes.Contains(got, want) {
			t.Fatalf("missing % x in % x", want, got)
		}
	}
}

func TestFeedIgnoresNonPositiveAndClampsToOneByte(t *testing.T) {
	if n := NewBuilder().Feed(0).Len(); n != 0 {
		t.Fatalf("Feed(0) wrote %d bytes, want 0", n)
	}
	got := NewBuilder().Feed(9999).Bytes()
	if got[2] != 255 {
		t.Fatalf("Feed(9999) count byte = %d, want 255", got[2])
	}
}

func TestBytesReturnsACopy(t *testing.T) {
	b := NewBuilder().Text("abc")
	first := b.Bytes()
	first[0] = 'z'

	if second := b.Bytes(); second[0] != 'a' {
		t.Fatalf("Bytes() aliased the internal buffer: got %q", second)
	}
}

func TestRuleUsesRequestedWidth(t *testing.T) {
	got := NewBuilder().Rule(Width80mm).Bytes()
	if want := Width80mm + 1; len(got) != want { // dashes + LF
		t.Fatalf("rule length = %d, want %d", len(got), want)
	}
}
