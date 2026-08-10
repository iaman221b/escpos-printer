package escpos

import (
	"bytes"
	"strings"
	"testing"
)

// The five commands must appear in this order. A printer that is told to print
// before the data is stored emits an empty symbol.
func TestQRCodeEmitsTheFiveCommandsInOrder(t *testing.T) {
	got := NewBuilder().QRCode("https://google.com").Bytes()

	steps := []struct {
		name  string
		bytes []byte
	}{
		{"select model", []byte{0x1d, 0x28, 0x6b, 4, 0, 49, 65, byte(QRModel2), 0}},
		{"module size", []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 67, DefaultQRModuleSize}},
		{"correction", []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 69, byte(QRCorrectionM)}},
		{"store data", []byte{0x1d, 0x28, 0x6b, byte(len("https://google.com") + 3), 0, 49, 80, 48}},
		{"print symbol", []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 81, 48}},
	}

	last := -1
	for _, step := range steps {
		at := bytes.Index(got, step.bytes)
		if at < 0 {
			t.Fatalf("%s command missing: % x", step.name, got)
		}
		if at < last {
			t.Fatalf("%s appears at %d, before the previous command at %d", step.name, at, last)
		}
		last = at
	}
}

func TestQRCodeCarriesTheDataVerbatim(t *testing.T) {
	const url = "https://google.com"
	got := NewBuilder().QRCode(url).Bytes()

	if !bytes.Contains(got, []byte(url)) {
		t.Fatalf("payload missing from the stream: % x", got)
	}
}

// The store command's length counts cn + fn + m as well as the payload, encoded
// little-endian across two bytes. Data over 253 bytes is what makes pH
// non-zero — a byte-order slip is invisible below that.
func TestQRStoreLengthIsLittleEndianAcrossBothBytes(t *testing.T) {
	data := strings.Repeat("A", 300)
	got := NewBuilder().QRCode(data).Bytes()

	n := len(data) + 3 // 303
	want := []byte{0x1d, 0x28, 0x6b, byte(n % 256), byte(n / 256), 49, 80, 48}

	if !bytes.Contains(got, want) {
		t.Fatalf("store header % x not found; the length field is wrong", want)
	}
	if want[3] != 47 || want[4] != 1 {
		t.Fatalf("test itself is wrong: expected pL=47 pH=1 for %d, got %d/%d", n, want[3], want[4])
	}
}

// Printing an empty symbol storage area makes some firmware emit a stray
// marker or stall, so nothing at all should be written.
func TestQRCodeEmptyDataEmitsNothing(t *testing.T) {
	if n := NewBuilder().QRCode("").Len(); n != 0 {
		t.Fatalf("empty QR wrote %d bytes, want 0", n)
	}
}

func TestQRCodeRejectsOversizedData(t *testing.T) {
	b := NewBuilder().QRCode(strings.Repeat("A", MaxQRDataLen+1))

	if b.Len() != 0 {
		t.Fatalf("oversized QR wrote %d bytes, want 0", b.Len())
	}
	if b.Err() == nil {
		t.Fatal("oversized QR recorded no error; a truncated payload would scan as the wrong value")
	}
}

func TestQRConfigZeroValueMatchesDefaults(t *testing.T) {
	withDefaults := NewBuilder().QRCode("x").Bytes()
	withZeroCfg := NewBuilder().QRCodeWith("x", QRConfig{}).Bytes()

	if !bytes.Equal(withDefaults, withZeroCfg) {
		t.Fatalf("QRConfig{} differs from the defaults:\n % x\n % x", withZeroCfg, withDefaults)
	}
}

func TestQRModuleSizeIsClamped(t *testing.T) {
	cases := []struct {
		given uint8
		want  uint8
	}{
		{0, DefaultQRModuleSize}, // zero means "unset", not "invisible"
		{99, maxQRModuleSize},
		{8, 8},
	}

	for _, c := range cases {
		got := NewBuilder().QRCodeWith("x", QRConfig{ModuleSize: c.given}).Bytes()
		want := []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 67, c.want}
		if !bytes.Contains(got, want) {
			t.Fatalf("module size %d did not clamp to %d", c.given, c.want)
		}
	}
}

func TestQRCorrectionAndModelAreSelectable(t *testing.T) {
	got := NewBuilder().QRCodeWith("x", QRConfig{
		Model:      QRModel1,
		Correction: QRCorrectionH,
	}).Bytes()

	if !bytes.Contains(got, []byte{0x1d, 0x28, 0x6b, 4, 0, 49, 65, byte(QRModel1), 0}) {
		t.Error("model 1 was not selected")
	}
	if !bytes.Contains(got, []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 69, byte(QRCorrectionH)}) {
		t.Error("correction level H was not selected")
	}
}

// The existing invariant must survive: whatever else is on the receipt, a cut
// always feeds the printed area clear of the blade first.
func TestQRFollowedByCutStillFeedsFirst(t *testing.T) {
	got := NewBuilder().QRCode("https://google.com").Cut().Bytes()

	feedAt := bytes.Index(got, []byte{0x1b, 0x64, DefaultCutFeed})
	cutAt := bytes.Index(got, []byte{0x1d, 0x56, 0x00})

	if feedAt < 0 || cutAt < 0 || feedAt > cutAt {
		t.Fatalf("feed must precede cut; feed=%d cut=%d", feedAt, cutAt)
	}
}
