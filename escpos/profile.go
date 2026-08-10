package escpos

// Profile describes the printer a document is being rendered for.
//
// It exists so a layout can be written once and print correctly on different
// hardware. Without it, every style ends up with the paper width baked into its
// format strings and the cutter gap baked into its cut — which is exactly how
// a second paper size turns into a second copy of the layout.
type Profile struct {
	// Name identifies the profile in logs and diagnostics.
	Name string

	// Columns is the character width of the roll in the default font. Row and
	// Rule lay out against it.
	Columns int

	// DotWidth is the printable width in dots. Used to size raster output —
	// images, and QR modules where a symbol must fit the roll.
	DotWidth int

	// CutFeed is how many lines to feed before cutting. See DefaultCutFeed for
	// why cutting without feeding produces a blank slip.
	CutFeed int

	// Caps says what the hardware can actually draw. A renderer skips an
	// element the printer cannot produce rather than emitting a command it will
	// render as garbage.
	Caps Capabilities
}

// Capabilities records which optional element kinds a printer supports.
//
// The zero value claims nothing, which is the safe default: a profile that has
// not been told a printer supports QR codes will not send QR commands to it.
type Capabilities struct {
	QR      bool
	Barcode bool
	Image   bool
}

// Profile58mm is the common 58mm thermal roll: 32 columns, 384 dots.
//
// The cut feed matches an Epson TM-T82X-II, whose print head sits roughly 15mm
// above the cutter.
var Profile58mm = Profile{
	Name:     "58mm",
	Columns:  Width58mm,
	DotWidth: 384,
	CutFeed:  DefaultCutFeed,
	Caps:     Capabilities{QR: true, Barcode: true, Image: true},
}

// Profile80mm is the common 80mm thermal roll: 48 columns, 576 dots.
var Profile80mm = Profile{
	Name:     "80mm",
	Columns:  Width80mm,
	DotWidth: 576,
	CutFeed:  DefaultCutFeed,
	Caps:     Capabilities{QR: true, Barcode: true, Image: true},
}

// normalize fills in anything a caller left at zero, so a hand-built Profile
// with only Columns set still renders rather than producing a zero-width
// document.
func (p Profile) normalize() Profile {
	if p.Columns <= 0 {
		p.Columns = Width58mm
	}
	if p.DotWidth <= 0 {
		// Thermal printers run at 8 dots/mm and the usual roll gives 12 dots
		// per character cell in the default font.
		p.DotWidth = p.Columns * 12
	}
	if p.CutFeed <= 0 {
		p.CutFeed = DefaultCutFeed
	}
	if p.Name == "" {
		p.Name = "custom"
	}
	return p
}
