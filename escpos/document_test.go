package escpos

import (
	"bytes"
	"strings"
	"testing"
)

// Row layout is what keeps a style independent of the paper size. The same
// document must lay out correctly at 32 and 48 columns.
func TestRowFlexWidthAdaptsToTheProfile(t *testing.T) {
	row := Row{Cells: []Cell{
		{Text: "Flat white"},
		{Text: "x2", Width: 4, Align: AlignRight},
		{Text: "7.00", Width: 7, Align: AlignRight},
	}}

	for _, p := range []Profile{Profile58mm, Profile80mm} {
		got := row.layout(p.Columns)
		if len(got) != p.Columns {
			t.Errorf("%s: line is %d chars, want %d: %q", p.Name, len(got), p.Columns, got)
		}
		if !strings.HasPrefix(got, "Flat white") {
			t.Errorf("%s: flexible cell lost its text: %q", p.Name, got)
		}
		if !strings.HasSuffix(got, "   7.00") {
			t.Errorf("%s: right-aligned cell did not reach the edge: %q", p.Name, got)
		}
	}
}

// A line that reflowed would break the alignment of every column after it, so
// overlong text is cut rather than wrapped.
func TestRowTruncatesRatherThanWrapping(t *testing.T) {
	row := Row{Cells: []Cell{
		{Text: "A very long product name that will not fit"},
		{Text: "9.99", Width: 7, Align: AlignRight},
	}}

	got := row.layout(Profile58mm.Columns)
	if len(got) != Profile58mm.Columns {
		t.Fatalf("line is %d chars, want %d: %q", len(got), Profile58mm.Columns, got)
	}
	if !strings.HasSuffix(got, "   9.99") {
		t.Fatalf("the fixed column was pushed off the line: %q", got)
	}
}

func TestRowHandlesMoreColumnsThanFit(t *testing.T) {
	var cells []Cell
	for i := 0; i < 40; i++ {
		cells = append(cells, Cell{Text: "x"})
	}
	// Must not panic or produce a negative-width repeat.
	if got := (Row{Cells: cells}).layout(Profile58mm.Columns); got == "" {
		t.Fatal("degenerate row produced nothing")
	}
}

func TestLeftRightPinsTheFigureToTheEdge(t *testing.T) {
	got := LeftRight("TOTAL", "13.65").layout(Profile58mm.Columns)
	if !strings.HasPrefix(got, "TOTAL") || !strings.HasSuffix(got, "13.65") {
		t.Fatalf("layout = %q", got)
	}
	if len(got) != Profile58mm.Columns {
		t.Fatalf("line is %d chars, want %d", len(got), Profile58mm.Columns)
	}
}

func sampleDoc() Document {
	return Document{
		Text{Value: "THE CORNER SHOP", Bold: true, Align: AlignCenter},
		Text{Value: "Order #ORD-1043", Align: AlignCenter},
		Rule{},
		Row{Cells: []Cell{
			{Text: "Flat white"},
			{Text: "x2", Width: 4, Align: AlignRight},
			{Text: "7.00", Width: 7, Align: AlignRight},
		}},
		Rule{},
		LeftRight("TOTAL", "7.00"),
		Feed{Lines: 1},
		QR{Data: "https://google.com", Align: AlignCenter},
		Cut{},
	}
}

func TestRenderProducesAWellFormedStream(t *testing.T) {
	got, err := Render(sampleDoc(), Profile58mm)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !bytes.HasPrefix(got, []byte{0x1b, 0x40}) {
		t.Error("stream does not start with ESC @ init")
	}
	for _, want := range []struct {
		name  string
		bytes []byte
	}{
		{"QR print command", []byte{0x1d, 0x28, 0x6b, 3, 0, 49, 81, 48}},
		{"feed before cut", []byte{0x1b, 0x64, byte(Profile58mm.CutFeed)}},
		{"full cut", []byte{0x1d, 0x56, 0x00}},
	} {
		if !bytes.Contains(got, want.bytes) {
			t.Errorf("missing %s", want.name)
		}
	}

	feedAt := bytes.Index(got, []byte{0x1b, 0x64, byte(Profile58mm.CutFeed)})
	cutAt := bytes.Index(got, []byte{0x1d, 0x56, 0x00})
	if feedAt > cutAt {
		t.Error("cut precedes the feed; the receipt would come out blank")
	}
}

// Cut distance comes from the Profile, so a printer with a different
// head-to-cutter gap needs no change to the style.
func TestRenderTakesTheCutFeedFromTheProfile(t *testing.T) {
	custom := Profile58mm
	custom.CutFeed = 3

	got, err := Render(Document{Cut{}}, custom)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Contains(got, []byte{0x1b, 0x64, 3}) {
		t.Fatalf("profile cut feed of 3 was not honoured: % x", got)
	}
}

// An element the hardware cannot draw is dropped and reported, never sent as a
// command the printer would render as garbage.
func TestRenderSkipsUnsupportedElementsAndReportsThem(t *testing.T) {
	noSymbols := Profile58mm
	noSymbols.Caps = Capabilities{}

	got, err := Render(Document{
		Text{Value: "HEADER"},
		QR{Data: "https://google.com"},
		Barcode{Data: "ORD-1"},
		Text{Value: "FOOTER"},
	}, noSymbols)

	if err == nil {
		t.Fatal("dropped elements were not reported")
	}
	if bytes.Contains(got, []byte{0x1d, 0x28, 0x6b}) {
		t.Error("a QR command was emitted to a profile that cannot print one")
	}
	if !bytes.Contains(got, []byte("FOOTER")) {
		t.Error("rendering stopped at the unsupported element")
	}
}

// A failed element must not cost the rest of the receipt.
func TestRenderReportsElementErrorsButStillReturnsBytes(t *testing.T) {
	got, err := Render(Document{
		Text{Value: "HEADER"},
		Barcode{Data: "nope", Format: EAN13},
		Text{Value: "TOTAL 25.00"},
	}, Profile58mm)

	if err == nil {
		t.Fatal("the invalid barcode was not reported")
	}
	if !bytes.Contains(got, []byte("TOTAL 25.00")) {
		t.Fatal("the receipt stopped rendering after the bad element")
	}
}

// The readable rendering is what makes a layout reviewable in a diff. Assert
// its shape rather than an exact golden here, so the library's own tests do not
// pin a consumer's receipt design.
func TestRenderTextIsReadable(t *testing.T) {
	got := RenderText(sampleDoc(), Profile58mm)

	for _, want := range []string{
		"**THE CORNER SHOP**",
		"Order #ORD-1043",
		strings.Repeat("-", Profile58mm.Columns),
		"Flat white",
		"TOTAL",
		"[QR: https://google.com]",
		"[CUT]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("readable output is missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderTextRespectsProfileWidth(t *testing.T) {
	strip := strings.NewReplacer("**", "", "_", "")

	for _, p := range []Profile{Profile58mm, Profile80mm} {
		out := RenderText(sampleDoc(), p)
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			// Emphasis and symbol markers are annotations, not printed content.
			if strings.HasPrefix(strings.TrimSpace(line), "[") {
				continue
			}
			if width := len(strip.Replace(line)); width > p.Columns {
				t.Errorf("%s: line exceeds %d columns (%d): %q", p.Name, p.Columns, width, line)
			}
		}
	}
}

// A line that exactly fills the roll must not lose its end to the emphasis
// markers — that would hide the total, which is the one figure anyone checks.
func TestRenderTextDoesNotTruncateEmphasisedFullWidthLines(t *testing.T) {
	full := strings.Repeat("X", Profile58mm.Columns)

	got := RenderText(Document{Text{Value: full, Bold: true}}, Profile58mm)

	if !strings.Contains(got, "**"+full+"**") {
		t.Fatalf("a full-width bold line was truncated:\n%s", got)
	}
}

// A totals row needs both the column alignment of a Row and the emphasis of a
// Text. Without Bold on Row, a style would have to preformat the line and
// hardcode the paper width back in.
func TestRowEmphasisAppliesToTheWholeLine(t *testing.T) {
	row := LeftRight("TOTAL", "13.65")
	row.Bold = true

	got, err := Render(Document{row}, Profile58mm)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	boldOn := bytes.Index(got, []byte{0x1b, 0x45, 0x01})
	total := bytes.Index(got, []byte("TOTAL"))
	boldOff := bytes.Index(got, []byte{0x1b, 0x45, 0x00})

	if boldOn < 0 || boldOn > total {
		t.Fatal("bold was not turned on before the row")
	}
	if boldOff < total {
		t.Fatal("bold was turned off before the row finished")
	}

	if text := RenderText(Document{row}, Profile58mm); !strings.Contains(text, "**TOTAL") {
		t.Fatalf("readable output does not mark the row as bold: %q", text)
	}
}

// A hand-built Profile with only Columns set must still render.
func TestProfileNormalizesPartialValues(t *testing.T) {
	got := Profile{Columns: 40}.normalize()

	if got.CutFeed != DefaultCutFeed {
		t.Errorf("CutFeed = %d, want the default %d", got.CutFeed, DefaultCutFeed)
	}
	if got.DotWidth <= 0 {
		t.Error("DotWidth was left at zero")
	}
	if got.Name == "" {
		t.Error("Name was left empty")
	}
}

func TestTextStyleDoesNotLeakToTheNextElement(t *testing.T) {
	got, err := Render(Document{
		Text{Value: "BOLD HEADER", Bold: true},
		Text{Value: "plain item"},
	}, Profile58mm)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	boldOff := bytes.Index(got, []byte{0x1b, 0x45, 0x00})
	plainAt := bytes.Index(got, []byte("plain item"))

	if boldOff < 0 || boldOff > plainAt {
		t.Fatal("bold was not turned off before the following line")
	}
}
