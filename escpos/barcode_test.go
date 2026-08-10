package escpos

import (
	"bytes"
	"strings"
	"testing"
)

// Without a code-set selector Epson firmware prints nothing at all. This is the
// single most common way hand-rolled Code128 fails, so it is supplied for the
// caller.
func TestCode128GetsACodeSetPrefix(t *testing.T) {
	got := NewBuilder().Barcode("ORD-1043").Bytes()

	if !bytes.Contains(got, []byte("{BORD-1043")) {
		t.Fatalf("Code128 payload was not prefixed with {B: %q", got)
	}
}

func TestCode128DoesNotDoubleAnExplicitPrefix(t *testing.T) {
	for _, prefix := range []string{"{A", "{B", "{C"} {
		got := NewBuilder().Barcode(prefix + "1234").Bytes()
		if bytes.Contains(got, []byte("{B"+prefix)) {
			t.Fatalf("prefix %s was doubled: %q", prefix, got)
		}
		if !bytes.Contains(got, []byte(prefix+"1234")) {
			t.Fatalf("explicit prefix %s was not preserved: %q", prefix, got)
		}
	}
}

func TestBarcodeEmitsTheCommandSequence(t *testing.T) {
	got := NewBuilder().Barcode("ORD-1").Bytes()

	want := [][]byte{
		{0x1d, 0x68, DefaultBarcodeHeight},   // GS h
		{0x1d, 0x77, DefaultBarcodeWidth},    // GS w
		{0x1d, 0x48, 2},                      // GS H — below the bars by default
		{0x1d, 0x66, 0},                      // GS f
		{0x1d, 0x6b, byte(Code128), byte(7)}, // GS k, len("{BORD-1")
	}
	for _, w := range want {
		if !bytes.Contains(got, w) {
			t.Fatalf("missing command % x in % x", w, got)
		}
	}
}

func TestBarcodeEmptyDataEmitsNothing(t *testing.T) {
	if n := NewBuilder().Barcode("").Len(); n != 0 {
		t.Fatalf("empty barcode wrote %d bytes, want 0", n)
	}
}

// A barcode that scans as the wrong value is worse than one that is missing,
// because nobody re-checks a barcode that printed.
func TestBarcodeRejectsDataTheSymbologyCannotEncode(t *testing.T) {
	cases := []struct {
		name   string
		format BarcodeFormat
		data   string
	}{
		{"EAN13 too short", EAN13, "123"},
		{"EAN13 non-digit", EAN13, "12345678901A"},
		{"EAN8 too long", EAN8, "123456789"},
		{"UPCA too short", UPCA, "123"},
		{"ITF odd length", ITF, "12345"},
		{"Code39 lowercase", Code39, "abc"},
		{"Codabar no guards", Codabar, "1234"},
		{"unknown format", BarcodeFormat(200), "1234"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := NewBuilder().BarcodeWith(c.data, BarcodeConfig{Format: c.format})
			if b.Len() != 0 {
				t.Errorf("wrote %d bytes for invalid data, want 0", b.Len())
			}
			if b.Err() == nil {
				t.Error("recorded no error for data the symbology cannot encode")
			}
		})
	}
}

func TestBarcodeAcceptsValidData(t *testing.T) {
	cases := []struct {
		name   string
		format BarcodeFormat
		data   string
	}{
		{"EAN13 12 digits", EAN13, "123456789012"},
		{"EAN13 13 digits", EAN13, "1234567890128"},
		{"EAN8", EAN8, "1234567"},
		{"UPCA", UPCA, "12345678901"},
		{"ITF even", ITF, "123456"},
		{"Code39 uppercase", Code39, "ORD-1043"},
		{"Codabar guarded", Codabar, "A1234B"},
		{"Code93", Code93, "ORD-1043"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := NewBuilder().BarcodeWith(c.data, BarcodeConfig{Format: c.format})
			if b.Err() != nil {
				t.Errorf("valid data rejected: %v", b.Err())
			}
			if b.Len() == 0 {
				t.Error("valid data produced no output")
			}
		})
	}
}

func TestBarcodeWidthIsClamped(t *testing.T) {
	for _, c := range []struct{ given, want uint8 }{
		{0, DefaultBarcodeWidth},
		{1, minBarcodeWidth},
		{99, maxBarcodeWidth},
		{4, 4},
	} {
		got := NewBuilder().BarcodeWith("ORD-1", BarcodeConfig{Width: c.given}).Bytes()
		if !bytes.Contains(got, []byte{0x1d, 0x77, c.want}) {
			t.Errorf("width %d did not clamp to %d", c.given, c.want)
		}
	}
}

func TestBarcodeRejectsOverlongData(t *testing.T) {
	b := NewBuilder().BarcodeWith(strings.Repeat("A", 300), BarcodeConfig{Format: Code93})
	if b.Err() == nil {
		t.Fatal("data past the single-byte length field was accepted")
	}
}

// The first error is kept and later elements still render — a bad barcode
// costs the barcode, not the receipt.
func TestBuilderKeepsTheFirstErrorAndKeepsGoing(t *testing.T) {
	b := NewBuilder().
		Line("HEADER").
		BarcodeWith("nope", BarcodeConfig{Format: EAN13}).
		Line("TOTAL 25.00")

	if b.Err() == nil {
		t.Fatal("no error recorded for the invalid barcode")
	}
	if !bytes.Contains(b.Bytes(), []byte("TOTAL 25.00")) {
		t.Fatal("the receipt stopped rendering after a failed element")
	}
}
