//go:build windows

package windows

import (
	"testing"
	"unsafe"

	"github.com/iaman221b/escpos-printer/device"
)

// printerInfo1 is walked by pointer arithmetic over the EnumPrintersW buffer.
// If the struct were laid out differently, ListQueues would read at the wrong
// stride and return garbage names — so its size is pinned here.
func TestPrinterInfo1Layout(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	want := unsafe.Sizeof(uint32(0)) + 3*ptr

	// Account for the alignment padding after Flags on 64-bit.
	if got := unsafe.Sizeof(printerInfo1{}); got < want {
		t.Fatalf("printerInfo1 size = %d, want at least %d", got, want)
	}
	if got := unsafe.Offsetof(printerInfo1{}.Name); got != unsafe.Offsetof(printerInfo1{}.Description)+ptr {
		t.Fatalf("Name offset = %d, not one pointer past Description", got)
	}
}

// The captured real-world case: an EPSON TM-T82X-II unplugged from USB001
// reports no status bits at all, and records the removal in Attributes.
func TestClassifyQueueStateDetectsUnpluggedUSB(t *testing.T) {
	online, paper, reason := classifyQueueState(0, 1092) // WORK_OFFLINE|LOCAL|DEFAULT

	if online {
		t.Fatal("an unplugged printer must not report as online")
	}
	if paper != device.PaperOK {
		t.Fatalf("paper = %q, want %q — no paper bits were set", paper, device.PaperOK)
	}
	if reason == "" {
		t.Fatal("an offline queue must explain itself")
	}
}

func TestClassifyQueueStateHealthy(t *testing.T) {
	online, paper, reason := classifyQueueState(0, 0)
	if !online || paper != device.PaperOK || reason != "" {
		t.Fatalf("classify(0,0) = (%v, %q, %q), want (true, ok, \"\")", online, paper, reason)
	}
}

func TestClassifyQueueStatePaperOut(t *testing.T) {
	_, paper, _ := classifyQueueState(statusPaperOut, 0)
	if paper != device.PaperOut {
		t.Fatalf("paper = %q, want %q", paper, device.PaperOut)
	}
}

func TestClassifyQueueStateDoorOpen(t *testing.T) {
	online, _, reason := classifyQueueState(statusDoorOpen, 0)
	if online {
		t.Fatal("an open cover must report offline")
	}
	if reason != coverOpenReason {
		t.Fatalf("reason = %q, want %q", reason, coverOpenReason)
	}
}

// Exercises the EnumPrintersW buffer walk against the real spooler. Any machine
// running this has at least one queue (Windows installs Print to PDF), but the
// assertion is deliberately weak: the point is that the walk does not crash or
// return corrupted strings.
func TestListQueues(t *testing.T) {
	for _, name := range ListQueues() {
		if name == "" {
			t.Fatal("ListQueues returned an empty name; the buffer walk is misaligned")
		}
	}
}
