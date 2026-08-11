package escposprinter

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/iaman221b/escpos-printer/device"
)

// Discovery: running every source, and deciding which printer ends up active.
//
// This is the part of the registry that is genuinely device-agnostic — the
// finders differ per platform and per device kind, but tolerating a failing
// source, ordering the results predictably, and applying the selection
// precedence are the same questions whatever is being discovered.

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
