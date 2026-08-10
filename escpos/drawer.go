package escpos

// The cash drawer lives in this package, next to the receipt bytes, because
// that is physically what it is.
//
// A retail cash drawer is almost never a peripheral in its own right: the usual
// wiring is an RJ-11/RJ-12 cable from the drawer into the receipt printer's DK
// port. So "open the drawer" means "send five bytes to the printer we already
// hold a Backend for" — it is a printer command, not a separate device.
//
// `ESC p m t1 t2` (0x1B 0x70 ...) is the ESC/POS "generate pulse" command, and
// it is what every receipt printer with a DK port understands. `m` selects the
// connector pin — 0 is pin 2, which is what a single-drawer setup uses; 1 is
// pin 5, used by the second drawer on a two-drawer till. `t1`/`t2` are the on
// and off durations in 2 ms units.
//
// The command is fire-and-forget. There is no read-back on the DK port: a
// printer accepts the kick identically whether or not a drawer is plugged into
// it, so nothing here can report that the drawer opened — only that the pulse
// was delivered. Any API that claims otherwise is inventing information.

const (
	// pulseUnitMs is the duration unit of the ESC/POS timing bytes.
	pulseUnitMs = 2
	// maxPulseMs is the longest pulse the single-byte timing fields encode.
	maxPulseMs = 255 * pulseUnitMs
)

// DrawerPulse is the solenoid pulse shape. The defaults suit the common
// single-drawer till and are overridable because a stiff or high-current
// drawer can need a longer pulse to actually throw the latch.
type DrawerPulse struct {
	// Pin selects the connector pin: 0 -> pin 2 (first drawer), 1 -> pin 5
	// (second drawer).
	Pin uint8 `json:"pin"`
	// OnMs is how long the solenoid is energised.
	OnMs uint32 `json:"onMs"`
	// OffMs is the settle time after the pulse.
	OffMs uint32 `json:"offMs"`
}

// DefaultPulse is the conventional kick for a single-drawer till.
func DefaultPulse() DrawerPulse {
	return DrawerPulse{Pin: 0, OnMs: 50, OffMs: 500}
}

// Normalize fills in zero values with the defaults. A caller that supplies a
// partially-filled pulse — or none at all — still produces a real kick rather
// than a degenerate zero-length one.
func (p DrawerPulse) Normalize() DrawerPulse {
	defaults := DefaultPulse()
	if p.OnMs == 0 {
		p.OnMs = defaults.OnMs
	}
	if p.OffMs == 0 {
		p.OffMs = defaults.OffMs
	}
	return p
}

// ToBytes encodes the pulse as `ESC p m t1 t2`.
func (p DrawerPulse) ToBytes() []byte {
	// Anything outside pin 2 is treated as pin 5 rather than rejected: the
	// value usually comes from stored configuration, and a bad one should still
	// kick a drawer instead of failing the sale.
	m := byte(0x00)
	if p.Pin != 0 {
		m = 0x01
	}
	return []byte{0x1B, 0x70, m, encodeDuration(p.OnMs), encodeDuration(p.OffMs)}
}

// encodeDuration converts milliseconds to the protocol's 2 ms units, clamped
// to what one byte holds. Floors to at least one unit so a mistakenly-zero
// configuration still emits a pulse.
func encodeDuration(millis uint32) byte {
	clamped := millis
	if clamped < pulseUnitMs {
		clamped = pulseUnitMs
	}
	if clamped > maxPulseMs {
		clamped = maxPulseMs
	}
	return byte(clamped / pulseUnitMs)
}
