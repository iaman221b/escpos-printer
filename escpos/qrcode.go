package escpos

import "fmt"

// QR codes are drawn by the printer's own firmware. The host sends the data
// with `GS ( k` and the printer encodes and rasterises the symbol itself, so
// nothing here has to implement QR encoding — and nothing should. Generating
// the symbol host-side would mean shipping a bitmap, which is slower to
// transmit and visibly coarser on thermal paper than what the firmware draws.
//
// The sequence is five commands, in order:
//
//	select model      GS ( k 04 00 49 65 n 00
//	set module size   GS ( k 03 00 49 67 n
//	set correction    GS ( k 03 00 49 69 n
//	store data        GS ( k pL pH 49 80 48 <data>
//	print symbol      GS ( k 03 00 49 81 48
//
// `pL pH` is a little-endian length. On the store command it counts cn + fn +
// m + the data, i.e. len(data)+3 — the one place in this file where an
// arithmetic slip produces a symbol that looks fine and will not scan.

// qrCn selects the QR code symbol family within GS ( k.
const qrCn = 49

// QRModel selects the symbol standard.
type QRModel uint8

const (
	// QRModel2 is the modern standard and what virtually every scanner and
	// phone camera expects. Default.
	QRModel2 QRModel = 50
	// QRModel1 is the original 1994 standard, kept for old fixed scanners.
	QRModel1 QRModel = 49
	// QRMicro is Micro QR: smaller, but poorly supported by phone cameras.
	QRMicro QRModel = 51
)

// QRErrorCorrection is how much of the symbol can be damaged and still read.
//
// Thermal print fades, smudges, and is often folded into a wallet, so the
// higher levels are worth the extra modules on a receipt.
type QRErrorCorrection uint8

const (
	QRCorrectionL QRErrorCorrection = 48 // 7%
	QRCorrectionM QRErrorCorrection = 49 // 15% — default
	QRCorrectionQ QRErrorCorrection = 50 // 25%
	QRCorrectionH QRErrorCorrection = 51 // 30%
)

const (
	// DefaultQRModuleSize is the width of one QR module in dots.
	//
	// At 58mm (384 dots) a short URL is a version 1-2 symbol of about 25
	// modules, so 6 dots gives roughly a 150-dot square: large enough for a
	// phone camera to read off thermal paper, small enough not to dominate the
	// receipt. Raise it before anything else if a printed code will not scan.
	DefaultQRModuleSize = 6

	// minQRModuleSize and maxQRModuleSize bound what the firmware accepts.
	minQRModuleSize = 1
	maxQRModuleSize = 16

	// MaxQRDataLen is the largest payload a QR symbol can hold, in the most
	// favourable (numeric) mode. Anything longer cannot be encoded at all, so
	// it is rejected rather than sent for the printer to refuse.
	MaxQRDataLen = 7089
)

// QRConfig tunes a QR code. The zero value is valid and means "the defaults":
// Model 2, module size 6, correction M.
type QRConfig struct {
	Model      QRModel
	ModuleSize uint8
	Correction QRErrorCorrection
}

// normalize fills in zero values and clamps the module size into range.
//
// Clamping rather than rejecting matches how DrawerPulse handles a bad stored
// value: configuration that has drifted out of range should still produce a
// scannable code instead of failing the sale.
func (c QRConfig) normalize() QRConfig {
	switch c.Model {
	case QRModel1, QRModel2, QRMicro:
	default:
		c.Model = QRModel2
	}

	switch c.Correction {
	case QRCorrectionL, QRCorrectionM, QRCorrectionQ, QRCorrectionH:
	default:
		c.Correction = QRCorrectionM
	}

	if c.ModuleSize == 0 {
		c.ModuleSize = DefaultQRModuleSize
	}
	if c.ModuleSize < minQRModuleSize {
		c.ModuleSize = minQRModuleSize
	}
	if c.ModuleSize > maxQRModuleSize {
		c.ModuleSize = maxQRModuleSize
	}

	return c
}

// QRCode appends a QR code with the default settings.
//
// Empty data is a no-op: printing the symbol storage area while it is empty
// makes some firmware emit a stray marker or stall, so nothing is emitted at
// all.
func (b *Builder) QRCode(data string) *Builder {
	return b.QRCodeWith(data, QRConfig{})
}

// QRCodeWith appends a QR code with explicit settings.
//
// Data longer than MaxQRDataLen records an error retrievable with Err and emits
// nothing — a truncated payload would produce a symbol that scans and yields
// the wrong value, which is worse than no symbol.
func (b *Builder) QRCodeWith(data string, cfg QRConfig) *Builder {
	if data == "" {
		return b
	}
	if len(data) > MaxQRDataLen {
		return b.fail(fmt.Errorf("qr data is %d bytes, over the %d-byte maximum", len(data), MaxQRDataLen))
	}

	cfg = cfg.normalize()

	// Select model.
	b.writeBytes([]byte{0x1d, 0x28, 0x6b, 4, 0, qrCn, 65, byte(cfg.Model), 0})
	// Set module size.
	b.writeBytes([]byte{0x1d, 0x28, 0x6b, 3, 0, qrCn, 67, cfg.ModuleSize})
	// Set error correction level.
	b.writeBytes([]byte{0x1d, 0x28, 0x6b, 3, 0, qrCn, 69, byte(cfg.Correction)})

	// Store the data. The length counts cn + fn + m as well as the payload.
	n := len(data) + 3
	b.writeBytes([]byte{0x1d, 0x28, 0x6b, byte(n % 256), byte(n / 256), qrCn, 80, 48})
	b.write(data)

	// Print what was stored.
	b.writeBytes([]byte{0x1d, 0x28, 0x6b, 3, 0, qrCn, 81, 48})

	return b
}
