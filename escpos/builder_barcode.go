package escpos

import (
	"fmt"
	"strings"
)

// Linear barcodes, like QR codes, are drawn by the printer's firmware: the host
// sends `GS k` with a symbology and the data, and the printer encodes the bars
// itself. There is no client-side encoding to do.
//
//	set height     GS h n
//	set width      GS w n
//	HRI position   GS H n
//	HRI font       GS f n
//	print          GS k m n d1..dn

// BarcodeFormat is a linear symbology.
//
// The values are the function-B selectors of `GS k`. Function B is used rather
// than the older NUL-terminated function A because it carries an explicit
// length, so the data may contain any byte.
type BarcodeFormat uint8

const (
	UPCA    BarcodeFormat = 65 // 11-12 digits
	UPCE    BarcodeFormat = 66 // 11-12 digits
	EAN13   BarcodeFormat = 67 // 12-13 digits
	EAN8    BarcodeFormat = 68 // 7-8 digits
	Code39  BarcodeFormat = 69 // digits, A-Z, space, $%+-./
	ITF     BarcodeFormat = 70 // digits, even count
	Codabar BarcodeFormat = 71 // digits and $+-./: between A-D start/stop
	Code93  BarcodeFormat = 72
	Code128 BarcodeFormat = 73 // any ASCII, needs a code-set prefix
)

// HRIPosition places the human-readable text that accompanies a barcode.
//
// These are deliberately not the wire values. The zero value has to mean "the
// caller did not choose", so that BarcodeConfig{} gets the sensible default —
// and the sensible default is the number printed under the bars, as on
// packaging, not no text at all. wire() maps to what GS H expects.
type HRIPosition uint8

const (
	// HRIDefault is the zero value: text below the bars.
	HRIDefault HRIPosition = iota
	HRINone
	HRIAbove
	HRIBelow
	HRIBoth
)

// wire returns the GS H parameter for this position.
func (h HRIPosition) wire() byte {
	switch h {
	case HRINone:
		return 0
	case HRIAbove:
		return 1
	case HRIBoth:
		return 3
	default: // HRIDefault, HRIBelow, and anything out of range
		return 2
	}
}

const (
	// DefaultBarcodeHeight is the bar height in dots. 162 is the firmware
	// default and prints about 20mm tall.
	DefaultBarcodeHeight = 162

	// DefaultBarcodeWidth is the module width. 2..6 are accepted; 3 is the
	// firmware default and fits a 12-character Code128 payload on 58mm paper.
	DefaultBarcodeWidth = 3

	minBarcodeWidth = 2
	maxBarcodeWidth = 6

	// maxBarcodeDataLen is what the single-byte length field of GS k holds.
	maxBarcodeDataLen = 255
)

// BarcodeConfig tunes a barcode. The zero value is valid and means "the
// defaults": Code128, height 162, width 3, human-readable text below.
type BarcodeConfig struct {
	Format BarcodeFormat
	Height uint8
	Width  uint8
	HRI    HRIPosition
}

func (c BarcodeConfig) normalize() BarcodeConfig {
	if c.Format == 0 {
		c.Format = Code128
	}
	if c.Height == 0 {
		c.Height = DefaultBarcodeHeight
	}
	if c.Width == 0 {
		c.Width = DefaultBarcodeWidth
	}
	// Clamped rather than rejected, as elsewhere: a configuration that has
	// drifted out of range should still print something scannable.
	if c.Width < minBarcodeWidth {
		c.Width = minBarcodeWidth
	}
	if c.Width > maxBarcodeWidth {
		c.Width = maxBarcodeWidth
	}
	return c
}

// Barcode appends a Code128 barcode with the default settings.
//
// Code128 is the right default for an order number or an internal SKU: it
// carries the full alphanumeric range, and it is one of the symbologies
// scanners are conventionally configured to read.
func (b *Builder) Barcode(data string) *Builder {
	return b.BarcodeWith(data, BarcodeConfig{})
}

// BarcodeWith appends a barcode with explicit settings.
//
// Data that the symbology cannot represent records an error retrievable with
// Err and emits nothing. Sending it anyway would either print nothing or print
// a symbol that scans as the wrong value — and a barcode that scans wrongly is
// worse than one that is missing, because nobody checks.
func (b *Builder) BarcodeWith(data string, cfg BarcodeConfig) *Builder {
	if data == "" {
		return b
	}

	cfg = cfg.normalize()

	encoded, err := encodeBarcodeData(cfg.Format, data)
	if err != nil {
		return b.fail(err)
	}
	if len(encoded) > maxBarcodeDataLen {
		return b.fail(fmt.Errorf("barcode data is %d bytes, over the %d-byte maximum",
			len(encoded), maxBarcodeDataLen))
	}

	b.writeBytes([]byte{0x1d, 0x68, cfg.Height})      // GS h — height
	b.writeBytes([]byte{0x1d, 0x77, cfg.Width})       // GS w — module width
	b.writeBytes([]byte{0x1d, 0x48, cfg.HRI.wire()})  // GS H — HRI position
	b.writeBytes([]byte{0x1d, 0x66, 0})               // GS f — HRI font A
	b.writeBytes([]byte{0x1d, 0x6b, byte(cfg.Format), // GS k m n <data>
		byte(len(encoded))})
	b.write(encoded)

	return b
}

// encodeBarcodeData validates the payload for the symbology and applies any
// prefix the firmware requires.
func encodeBarcodeData(format BarcodeFormat, data string) (string, error) {
	switch format {
	case Code128:
		// Epson firmware requires the payload to start with a code-set
		// selector: {A (control chars), {B (full alphanumeric), or {C (pairs
		// of digits). Without one the printer prints nothing at all, which is
		// the single most common way hand-rolled Code128 fails — so the
		// alphanumeric set is supplied when the caller has not chosen one.
		if len(data) >= 2 && data[0] == '{' && (data[1] == 'A' || data[1] == 'B' || data[1] == 'C') {
			return data, nil
		}
		return "{B" + data, nil

	case EAN13:
		return data, requireDigits(format, data, 12, 13)
	case EAN8:
		return data, requireDigits(format, data, 7, 8)
	case UPCA, UPCE:
		return data, requireDigits(format, data, 11, 12)

	case ITF:
		if err := requireDigits(format, data, 2, maxBarcodeDataLen); err != nil {
			return "", err
		}
		// ITF encodes digits in pairs, so an odd count cannot be represented.
		if len(data)%2 != 0 {
			return "", fmt.Errorf("ITF needs an even number of digits, got %d", len(data))
		}
		return data, nil

	case Code39:
		const allowed = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ -$%+./*"
		if i := strings.IndexFunc(data, func(r rune) bool {
			return !strings.ContainsRune(allowed, r)
		}); i >= 0 {
			return "", fmt.Errorf("Code39 cannot encode %q at position %d", data[i], i)
		}
		return data, nil

	case Codabar:
		// The start and stop characters are part of the payload for this
		// symbology, which callers routinely forget.
		if len(data) < 3 || !isCodabarGuard(data[0]) || !isCodabarGuard(data[len(data)-1]) {
			return "", fmt.Errorf("Codabar data must start and end with A, B, C or D")
		}
		return data, nil

	case Code93:
		return data, nil

	default:
		return "", fmt.Errorf("unknown barcode format %d", format)
	}
}

func requireDigits(format BarcodeFormat, data string, min, max int) error {
	if len(data) < min || len(data) > max {
		return fmt.Errorf("barcode format %d needs %d-%d digits, got %d", format, min, max, len(data))
	}
	for i := 0; i < len(data); i++ {
		if data[i] < '0' || data[i] > '9' {
			return fmt.Errorf("barcode format %d accepts digits only, found %q at position %d",
				format, data[i], i)
		}
	}
	return nil
}

func isCodabarGuard(c byte) bool {
	return c == 'A' || c == 'B' || c == 'C' || c == 'D' ||
		c == 'a' || c == 'b' || c == 'c' || c == 'd'
}
