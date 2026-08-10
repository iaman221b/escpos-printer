package escposprinter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/iaman221b/escpos-printer/device"
)

// Registry holds every printer currently visible, and remembers which one is
// active.
//
// Why a list rather than a single printer: one process can serve several tills.
// Each till has its own printer, and its own cash drawer cabled into that
// printer — so "the printer" is a question that only has an answer once you say
// which till is asking. A single-printer install is simply a list of one, and a
// request that names no printer goes to the active one.
//
// A Registry is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	printers map[string]*Printer
	devices  map[string]device.Device
	order    []string // discovery order, so the list renders predictably
	activeID string

	// pinnedID is a printer named explicitly by the application. It outranks
	// both the remembered selection and the automatic pick.
	pinnedID string

	finders []device.Finder
	store   SelectionStore
	logger  *slog.Logger
}

// New builds a registry with the discovery sources that make sense on this
// platform, plus whatever the options add.
//
// Discovery is not run here. Call Discover once the application is up, so a
// slow network sweep cannot delay startup.
func New(opts ...Option) *Registry {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	return &Registry{
		printers: make(map[string]*Printer),
		devices:  make(map[string]device.Device),
		pinnedID: cfg.pinnedID,
		finders:  cfg.finders(),
		store:    cfg.store,
		logger:   cfg.resolvedLogger(),
	}
}

// Sources names the discovery sources this registry will ask, for diagnostics.
func (r *Registry) Sources() []string {
	names := make([]string, 0, len(r.finders))
	for _, f := range r.finders {
		names = append(names, f.Name())
	}
	return names
}

// Pin names the printer that must be active whenever it is present. An empty
// id clears any pin. Usually set once through WithPin instead.
func (r *Registry) Pin(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pinnedID = id
}

// Add registers a printer directly, outside discovery. Adding the first printer
// makes it active, so a caller wiring up a single known device needs no
// selection step.
func (r *Registry) Add(found device.Discovered) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addLocked(found)

	if r.activeID == "" {
		r.activeID = found.Device.ID
	}
}

// addLocked records a printer without touching the selection. Discovery decides
// what is active deliberately once every printer is in, rather than inheriting
// whichever one happened to be enumerated first.
func (r *Registry) addLocked(found device.Discovered) {
	id := found.Device.ID
	if id == "" {
		return
	}

	if _, existed := r.printers[id]; !existed {
		r.order = append(r.order, id)
	}

	r.printers[id] = NewPrinter(found.Device, found.Backend)
	r.devices[id] = found.Device
}

// Devices lists what the registry currently holds, in discovery order.
func (r *Registry) Devices() []device.Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	devices := make([]device.Device, 0, len(r.order))
	for _, id := range r.order {
		if d, ok := r.devices[id]; ok {
			devices = append(devices, d)
		}
	}
	return devices
}

// ActiveID returns the selected printer's ID, or "" when no printer is
// available or a pinned printer is missing.
func (r *Registry) ActiveID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeID
}

// Printer returns a handle to one printer. An empty id means "the active one",
// which is what a print request that does not name a printer gets.
//
// The error is the point: when no printer is available, callers are told so
// with ErrNoPrinter instead of being handed something that pretends to print.
func (r *Registry) Printer(id string) (*Printer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if id == "" {
		if r.activeID == "" {
			return nil, device.ErrNoPrinter
		}
		id = r.activeID
	}

	p, ok := r.printers[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", device.ErrUnknownPrinter, id)
	}
	return p, nil
}

// Active returns the currently selected printer, or ErrNoPrinter.
func (r *Registry) Active() (*Printer, error) { return r.Printer("") }

// Select makes a printer active and remembers the choice.
func (r *Registry) Select(ctx context.Context, id string) error {
	r.mu.Lock()
	if _, ok := r.printers[id]; !ok {
		r.mu.Unlock()
		return fmt.Errorf("%w: %q", device.ErrUnknownPrinter, id)
	}
	r.activeID = id
	store := r.store
	r.mu.Unlock()

	r.logger.Info("active printer selected", "printerId", id)

	if store == nil {
		return nil
	}
	if err := store.SaveSelectedPrinterID(ctx, id); err != nil {
		// The selection has already taken effect in memory; failing to write it
		// down costs the choice on next boot, not this shift's printing. The
		// error is returned so the caller can surface it, but the printer is
		// already selected either way.
		r.logger.Error("could not persist the printer selection", "printerId", id, "error", err)
		return err
	}
	return nil
}

// Discover asks every source what it can see and rebuilds the list.
//
// A source that fails is logged, collected into the returned error, and
// skipped: a machine with no CUPS running must still find its network printers.
// Devices are returned regardless, so a partial failure is still a usable
// result — callers that do not care can ignore the error.
//
// The previously selected printer is re-activated if it is still present, so a
// rescan never silently moves printing to a different device.
func (r *Registry) Discover(ctx context.Context) ([]device.Device, error) {
	var (
		found       []device.Discovered
		failures    []error
		findersUsed int
	)

	for _, finder := range r.finders {
		devices, err := finder.Find(ctx)
		if err != nil {
			r.logger.Warn("printer discovery failed for one source",
				"finder", finder.Name(), "error", err)
			failures = append(failures, fmt.Errorf("%s: %w", finder.Name(), err))
			continue
		}
		findersUsed++
		found = append(found, devices...)
	}

	// Stable ordering: receipt printers first (the ones an operator is looking
	// for), then by name, so the list does not reshuffle between scans.
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Device.Receipt != found[j].Device.Receipt {
			return found[i].Device.Receipt
		}
		return found[i].Device.Name < found[j].Device.Name
	})

	r.mu.Lock()
	previous := r.activeID
	r.printers = make(map[string]*Printer, len(found))
	r.devices = make(map[string]device.Device, len(found))
	r.order = nil
	r.activeID = ""

	for _, d := range found {
		r.addLocked(d)
	}

	// Keep the operator's printer if it is still attached. Otherwise the
	// automatic pick below stands.
	if _, stillPresent := r.printers[previous]; stillPresent {
		r.activeID = previous
	} else if previous != "" {
		r.logger.Warn("previously selected printer is no longer present", "printerId", previous)
	}

	// An explicitly pinned printer wins outright — including over a selection
	// made on a settings screen, because the pin is the more deliberate
	// statement of the two.
	if r.pinnedID != "" {
		if _, present := r.printers[r.pinnedID]; present {
			r.activeID = r.pinnedID
		} else {
			// Deliberately left with nothing selected. A terminal that named
			// its printer must not quietly start printing somewhere else when
			// that printer is missing — receipts surfacing at the wrong counter
			// is worse than receipts not surfacing at all, because nobody
			// notices it happening.
			r.activeID = ""
			r.logger.Error("the pinned printer was not found; printing is disabled until it returns",
				"printerId", r.pinnedID)
		}
	} else {
		r.autoSelectLocked()
	}
	active := r.activeID
	r.mu.Unlock()

	r.logger.Info("printer discovery complete",
		"found", len(found), "sources", findersUsed, "active", active)

	return r.Devices(), errors.Join(failures...)
}

// autoSelectLocked picks a printer when nothing is selected: the single
// recognised receipt printer if there is exactly one, otherwise nothing.
//
// Deliberately conservative. Guessing between two candidates risks sending
// receipts to the wrong counter, which is worse than asking the operator once.
func (r *Registry) autoSelectLocked() {
	if r.activeID != "" {
		return
	}

	var candidates []string
	for _, id := range r.order {
		if r.devices[id].Receipt {
			candidates = append(candidates, id)
		}
	}

	if len(candidates) == 1 {
		r.activeID = candidates[0]
		r.logger.Info("single receipt printer found; selected automatically", "printerId", r.activeID)
		return
	}

	// No receipt printer recognised, but exactly one printer of any kind: use
	// it rather than leaving a working terminal with nothing selected.
	if len(candidates) == 0 && len(r.order) == 1 {
		r.activeID = r.order[0]
		r.logger.Info("one printer found; selected automatically", "printerId", r.activeID)
	}
}

// RestoreSelection re-applies the printer chosen on a previous run. Call it
// after the first Discover.
//
// It quietly does nothing when there is no store, when nothing was stored, or
// when the stored printer is no longer attached — the automatic pick already
// covers that case. A pinned printer is never overridden by an older on-screen
// choice.
func (r *Registry) RestoreSelection(ctx context.Context) error {
	if r.store == nil {
		return nil
	}

	id, err := r.store.LoadSelectedPrinterID(ctx)
	if err != nil {
		r.logger.Warn("could not read the remembered printer selection", "error", err)
		return err
	}
	if id == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pinnedID != "" {
		return nil
	}

	if _, ok := r.printers[id]; !ok {
		r.logger.Warn("remembered printer is not attached; keeping the automatic pick",
			"printerId", id, "active", r.activeID)
		return nil
	}

	r.activeID = id
	r.logger.Info("restored the remembered printer selection", "printerId", id)
	return nil
}
