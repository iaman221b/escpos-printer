//go:build !windows

package cups

import (
	"errors"
	"os/exec"
	"testing"
)

func TestNoCUPSPrintersRecognisesAnEmptySpooler(t *testing.T) {
	if !noCUPSPrinters(errors.New("exit status 1"), "lpstat: No destinations added.") {
		t.Fatal("an empty spooler should be quiet, not a warning")
	}
}

func TestNoCUPSPrintersRecognisesAMissingBinary(t *testing.T) {
	if !noCUPSPrinters(exec.ErrNotFound, "") {
		t.Fatal("a host without CUPS installed should be quiet")
	}
}

func TestNoCUPSPrintersRecognisesAWrappedMissingBinary(t *testing.T) {
	wrapped := &exec.Error{Name: "lpstat", Err: exec.ErrNotFound}
	if !noCUPSPrinters(wrapped, "") {
		t.Fatal("exec.ErrNotFound must be detected through the wrapper")
	}
}

func TestNoCUPSPrintersReportsGenuineFailure(t *testing.T) {
	if noCUPSPrinters(errors.New("exit status 1"), "lpstat: Transport endpoint is not connected") {
		t.Fatal("a broken cupsd must be reported, not swallowed")
	}
}

func TestParseLpstatLine(t *testing.T) {
	cases := []struct {
		line        string
		wantName    string
		wantEnabled bool
		wantOK      bool
	}{
		{"printer TM-T82X-II is idle.  enabled since Tue 03 Feb 2026 09:12:01", "TM-T82X-II", true, true},
		{"printer Office-Laser disabled since Mon 02 Feb 2026 17:40:11 -", "Office-Laser", false, true},
		{"\treason unknown", "", false, false},
		{"", "", false, false},
		{"device for TM-T82X-II: usb://EPSON", "", false, false},
	}

	for _, c := range cases {
		name, enabled, ok := parseLpstatLine(c.line)
		if ok != c.wantOK {
			t.Fatalf("parse(%q) ok = %v, want %v", c.line, ok, c.wantOK)
		}
		if !ok {
			continue
		}
		if name != c.wantName || enabled != c.wantEnabled {
			t.Fatalf("parse(%q) = (%q, %v), want (%q, %v)", c.line, name, enabled, c.wantName, c.wantEnabled)
		}
	}
}
