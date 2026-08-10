package escpos

import "testing"

func TestDefaultPulseBytes(t *testing.T) {
	got := DefaultPulse().ToBytes()
	want := []byte{0x1B, 0x70, 0x00, 25, 250} // 50ms/2 = 25, 500ms/2 = 250

	if len(got) != len(want) {
		t.Fatalf("pulse length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pulse = % x, want % x", got, want)
		}
	}
}

func TestPulsePinFive(t *testing.T) {
	got := DrawerPulse{Pin: 1, OnMs: 50, OffMs: 500}.ToBytes()
	if got[2] != 0x01 {
		t.Fatalf("pin byte = %#x, want 0x01", got[2])
	}
}

func TestPulsePinOutOfRangeTreatedAsPinFive(t *testing.T) {
	// A bad stored value must still kick a drawer rather than fail the sale.
	got := DrawerPulse{Pin: 9, OnMs: 50, OffMs: 500}.ToBytes()
	if got[2] != 0x01 {
		t.Fatalf("pin byte = %#x, want 0x01", got[2])
	}
}

func TestPulseDurationClampedToOneByte(t *testing.T) {
	got := DrawerPulse{OnMs: 100000, OffMs: 100000}.ToBytes()
	if got[3] != 255 || got[4] != 255 {
		t.Fatalf("durations = %d/%d, want 255/255", got[3], got[4])
	}
}

func TestPulseZeroDurationStillEmitsAPulse(t *testing.T) {
	// Normalize fills the defaults; encodeDuration floors at one unit even
	// without it. Both paths must produce a non-zero pulse.
	direct := DrawerPulse{OnMs: 0, OffMs: 0}.ToBytes()
	if direct[3] == 0 || direct[4] == 0 {
		t.Fatalf("durations = %d/%d, want non-zero", direct[3], direct[4])
	}
}

func TestNormalizeFillsMissingFields(t *testing.T) {
	got := DrawerPulse{Pin: 1}.Normalize()
	want := DefaultPulse()

	if got.OnMs != want.OnMs || got.OffMs != want.OffMs {
		t.Fatalf("normalized = %+v, want OnMs=%d OffMs=%d", got, want.OnMs, want.OffMs)
	}
	if got.Pin != 1 {
		t.Fatalf("Normalize overwrote Pin: got %d, want 1", got.Pin)
	}
}

func TestNormalizeKeepsExplicitValues(t *testing.T) {
	got := DrawerPulse{Pin: 0, OnMs: 120, OffMs: 80}.Normalize()
	if got.OnMs != 120 || got.OffMs != 80 {
		t.Fatalf("normalized = %+v, want 120/80", got)
	}
}
