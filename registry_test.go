package escposprinter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/escpos"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeBackend records what it was asked to print.
type fakeBackend struct {
	name     string
	written  [][]byte
	failWith error
	status   device.Status
}

func (f *fakeBackend) Name() string { return f.name }

func (f *fakeBackend) Print(ctx context.Context, data []byte) error {
	if f.failWith != nil {
		return f.failWith
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	f.written = append(f.written, cp)
	return nil
}

func (f *fakeBackend) Status(ctx context.Context) (device.Status, error) {
	return f.status, nil
}

// staticFinder returns a fixed set of devices, or an error.
type staticFinder struct {
	name  string
	found []device.Discovered
	err   error
}

func (s *staticFinder) Name() string { return s.name }

func (s *staticFinder) Find(ctx context.Context) ([]device.Discovered, error) {
	return s.found, s.err
}

func discovered(id, name string, receipt bool) device.Discovered {
	return device.Discovered{
		Device: device.Device{
			ID: id, Name: name, Receipt: receipt,
			Connection: device.ConnectionUSB, Transport: "test",
		},
		Backend: &fakeBackend{name: id, status: device.Status{Online: true, Paper: device.PaperOK}},
	}
}

func newTestRegistry(t *testing.T, finders ...device.Finder) *Registry {
	t.Helper()
	return New(WithFinders(finders...), WithLogger(quiet()))
}

func TestNoPrintersMeansErrNoPrinter(t *testing.T) {
	reg := newTestRegistry(t, &staticFinder{name: "empty"})
	if _, err := reg.Discover(context.Background()); err != nil {
		t.Fatalf("discover: %v", err)
	}

	_, err := reg.Active()
	if !errors.Is(err, ErrNoPrinter) {
		t.Fatalf("error = %v, want ErrNoPrinter", err)
	}
}

func TestUnknownPrinterIsDistinctFromNoPrinter(t *testing.T) {
	reg := newTestRegistry(t, &staticFinder{
		name:  "one",
		found: []device.Discovered{discovered("test:a", "Epson TM-T82", true)},
	})
	reg.Discover(context.Background())

	_, err := reg.Printer("test:missing")
	if !errors.Is(err, ErrUnknownPrinter) {
		t.Fatalf("error = %v, want ErrUnknownPrinter", err)
	}
}

func TestSingleReceiptPrinterIsSelectedAutomatically(t *testing.T) {
	reg := newTestRegistry(t, &staticFinder{
		name: "one",
		found: []device.Discovered{
			discovered("test:laser", "Office Laser", false),
			discovered("test:epson", "Epson TM-T82", true),
		},
	})
	reg.Discover(context.Background())

	if got := reg.ActiveID(); got != "test:epson" {
		t.Fatalf("active = %q, want the receipt printer", got)
	}
}

func TestTwoReceiptPrintersSelectNothing(t *testing.T) {
	// Guessing between two risks receipts at the wrong counter.
	reg := newTestRegistry(t, &staticFinder{
		name: "two",
		found: []device.Discovered{
			discovered("test:a", "Epson TM-T82", true),
			discovered("test:b", "Star TSP143", true),
		},
	})
	reg.Discover(context.Background())

	if got := reg.ActiveID(); got != "" {
		t.Fatalf("active = %q, want nothing selected", got)
	}
}

func TestSingleNonReceiptPrinterIsStillSelected(t *testing.T) {
	reg := newTestRegistry(t, &staticFinder{
		name:  "one",
		found: []device.Discovered{discovered("test:x", "Unknown Device", false)},
	})
	reg.Discover(context.Background())

	if got := reg.ActiveID(); got != "test:x" {
		t.Fatalf("active = %q, want the only printer", got)
	}
}

func TestPinOutranksAutomaticPick(t *testing.T) {
	reg := New(
		WithFinders(&staticFinder{
			name: "two",
			found: []device.Discovered{
				discovered("test:a", "Epson TM-T82", true),
				discovered("test:b", "Star TSP143", true),
			},
		}),
		WithPin("test:b"),
		WithLogger(quiet()),
	)
	reg.Discover(context.Background())

	if got := reg.ActiveID(); got != "test:b" {
		t.Fatalf("active = %q, want the pinned printer", got)
	}
}

// The rule that matters most: a terminal that named its printer must not
// quietly print somewhere else when that printer is missing.
func TestMissingPinDisablesPrintingRatherThanFallingBack(t *testing.T) {
	reg := New(
		WithFinders(&staticFinder{
			name:  "one",
			found: []device.Discovered{discovered("test:a", "Epson TM-T82", true)},
		}),
		WithPin("test:not-here"),
		WithLogger(quiet()),
	)
	reg.Discover(context.Background())

	if got := reg.ActiveID(); got != "" {
		t.Fatalf("active = %q, want nothing — a missing pin must not fall back", got)
	}
	if _, err := reg.Active(); !errors.Is(err, ErrNoPrinter) {
		t.Fatalf("error = %v, want ErrNoPrinter", err)
	}
}

func TestOneFailingFinderDoesNotBlindTheOthers(t *testing.T) {
	reg := newTestRegistry(t,
		&staticFinder{name: "broken", err: errors.New("cupsd is not running")},
		&staticFinder{name: "ok", found: []device.Discovered{discovered("test:a", "Epson TM-T82", true)}},
	)

	devices, err := reg.Discover(context.Background())
	if len(devices) != 1 {
		t.Fatalf("found %d devices, want 1 despite the failing source", len(devices))
	}
	if err == nil {
		t.Fatal("the failing source should be reported in the returned error")
	}
	if reg.ActiveID() != "test:a" {
		t.Fatalf("active = %q, want test:a", reg.ActiveID())
	}
}

func TestSelectionSurvivesARescan(t *testing.T) {
	finder := &staticFinder{
		name: "two",
		found: []device.Discovered{
			discovered("test:a", "Epson TM-T82", true),
			discovered("test:b", "Star TSP143", true),
		},
	}
	reg := newTestRegistry(t, finder)
	ctx := context.Background()

	reg.Discover(ctx)
	if err := reg.Select(ctx, "test:b"); err != nil {
		t.Fatalf("select: %v", err)
	}

	reg.Discover(ctx)
	if got := reg.ActiveID(); got != "test:b" {
		t.Fatalf("active = %q after rescan, want the operator's choice preserved", got)
	}
}

func TestSelectionIsPersistedAndRestored(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	finders := []device.Finder{&staticFinder{
		name: "two",
		found: []device.Discovered{
			discovered("test:a", "Epson TM-T82", true),
			discovered("test:b", "Star TSP143", true),
		},
	}}

	first := New(WithFinders(finders...), WithStore(store), WithLogger(quiet()))
	first.Discover(ctx)
	first.Select(ctx, "test:b")

	// A fresh registry, as after a restart.
	second := New(WithFinders(finders...), WithStore(store), WithLogger(quiet()))
	second.Discover(ctx)
	if err := second.RestoreSelection(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if got := second.ActiveID(); got != "test:b" {
		t.Fatalf("active = %q after restart, want test:b", got)
	}
}

func TestRestoreIgnoresAPrinterThatIsGone(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	store.SaveSelectedPrinterID(ctx, "test:vanished")

	reg := New(
		WithFinders(&staticFinder{
			name:  "one",
			found: []device.Discovered{discovered("test:a", "Epson TM-T82", true)},
		}),
		WithStore(store),
		WithLogger(quiet()),
	)
	reg.Discover(ctx)
	reg.RestoreSelection(ctx)

	if got := reg.ActiveID(); got != "test:a" {
		t.Fatalf("active = %q, want the automatic pick to stand", got)
	}
}

func TestPrintReachesTheActiveBackend(t *testing.T) {
	backend := &fakeBackend{name: "test:a"}
	reg := newTestRegistry(t, &staticFinder{
		name: "one",
		found: []device.Discovered{{
			Device:  device.Device{ID: "test:a", Name: "Epson TM-T82", Receipt: true},
			Backend: backend,
		}},
	})
	ctx := context.Background()
	reg.Discover(ctx)

	p, err := reg.Active()
	if err != nil {
		t.Fatalf("active: %v", err)
	}

	data := escpos.NewBuilder().Init().Line("TOTAL 25.00").Cut().Bytes()
	if err := p.Print(ctx, data); err != nil {
		t.Fatalf("print: %v", err)
	}

	if len(backend.written) != 1 {
		t.Fatalf("backend received %d jobs, want 1", len(backend.written))
	}
	if len(backend.written[0]) != len(data) {
		t.Fatalf("backend received %d bytes, want %d", len(backend.written[0]), len(data))
	}
}

func TestOpenDrawerSendsThePulseToThePrinter(t *testing.T) {
	backend := &fakeBackend{name: "test:a"}
	reg := newTestRegistry(t, &staticFinder{
		name: "one",
		found: []device.Discovered{{
			Device:  device.Device{ID: "test:a", Name: "Epson TM-T82", Receipt: true},
			Backend: backend,
		}},
	})
	ctx := context.Background()
	reg.Discover(ctx)

	p, _ := reg.Active()
	if err := p.OpenDrawer(ctx, escpos.DefaultPulse()); err != nil {
		t.Fatalf("open drawer: %v", err)
	}

	if len(backend.written) != 1 {
		t.Fatalf("backend received %d jobs, want 1", len(backend.written))
	}
	got := backend.written[0]
	if len(got) != 5 || got[0] != 0x1B || got[1] != 0x70 {
		t.Fatalf("drawer pulse = % x, want ESC p ...", got)
	}
}

func TestDrawerFailureIsReportedAsALinkFailure(t *testing.T) {
	backend := &fakeBackend{name: "test:a", failWith: device.ErrDisconnected}
	reg := newTestRegistry(t, &staticFinder{
		name: "one",
		found: []device.Discovered{{
			Device:  device.Device{ID: "test:a", Name: "Epson TM-T82", Receipt: true},
			Backend: backend,
		}},
	})
	ctx := context.Background()
	reg.Discover(ctx)

	p, _ := reg.Active()
	err := p.OpenDrawer(ctx, escpos.DefaultPulse())

	// The DK port has no read-back, so a failure is always the printer link —
	// never a claim about the drawer's mechanism.
	if !errors.Is(err, ErrDisconnected) {
		t.Fatalf("error = %v, want ErrDisconnected", err)
	}
}

func TestVirtualQueuesAreNeverTreatedAsReceiptPrinters(t *testing.T) {
	for _, name := range []string{"Microsoft Print to PDF", "Fax", "OneNote", "Adobe PDF"} {
		if device.LooksLikeReceiptPrinter(name) {
			t.Fatalf("%q was treated as a receipt printer", name)
		}
	}
}

func TestKnownVendorsAreRecognised(t *testing.T) {
	for _, name := range []string{"EPSON TM-T82X-II", "Star TSP143", "BIXOLON SRP-350"} {
		if !device.LooksLikeReceiptPrinter(name) {
			t.Fatalf("%q was not recognised as a receipt printer", name)
		}
	}
}

func TestSourcesNamesEveryFinder(t *testing.T) {
	reg := newTestRegistry(t,
		&staticFinder{name: "alpha"},
		&staticFinder{name: "beta"},
	)
	got := reg.Sources()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("sources = %v, want [alpha beta]", got)
	}
}
