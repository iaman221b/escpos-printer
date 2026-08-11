//go:build windows

package windows

import (
	"context"
	"errors"
	"testing"

	"github.com/iaman221b/escpos-printer/device"
)

func TestSentinelForMapsConditions(t *testing.T) {
	if got := sentinelFor(device.PaperOut, "whatever"); !errors.Is(got, device.ErrPaperOut) {
		t.Fatalf("paper-out did not map to ErrPaperOut: %v", got)
	}
	if got := sentinelFor(device.PaperOK, coverOpenReason); !errors.Is(got, device.ErrCoverOpen) {
		t.Fatalf("cover-open did not map to ErrCoverOpen: %v", got)
	}
	if got := sentinelFor(device.PaperOK, "paused"); !errors.Is(got, device.ErrDisconnected) {
		t.Fatalf("default did not map to ErrDisconnected: %v", got)
	}
}

func TestPrintRefusesEmptyPayload(t *testing.T) {
	b := NewSpoolerBackend("any")
	if err := b.Print(context.Background(), nil); err == nil {
		t.Fatal("printing zero bytes should be refused")
	}
}

func TestMissingQueueReportsDisconnected(t *testing.T) {
	b := NewSpoolerBackend("definitely-not-a-real-queue-9d3f1a")
	err := b.Print(context.Background(), []byte("x"))

	if err == nil {
		t.Fatal("printing to a nonexistent queue should fail")
	}
	if !errors.Is(err, device.ErrDisconnected) {
		t.Fatalf("error = %v, want it to wrap ErrDisconnected", err)
	}
}
