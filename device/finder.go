package device

import "context"

// Discovered pairs a described device with a backend ready to print to it.
//
// The pairing is the reason discovery cannot be generic: producing a Backend
// requires knowing what kind of device was found, so every Finder is specific
// to the mechanism it searches.
type Discovered struct {
	Device  Device
	Backend Backend
}

// Finder answers "what printers can you see right now?" for one mechanism —
// the Windows spooler, CUPS, USB device nodes, the network. Each is
// independent, and the platform-specific ones are behind build tags, so a
// finder that cannot work on a platform is simply not compiled into it.
//
// A finder that finds nothing returns an empty slice and no error. An error
// means the question could not be asked (the print service is not running),
// which is worth reporting but must never stop the other finders.
type Finder interface {
	// Name identifies the finder in logs and in discovery diagnostics.
	Name() string
	Find(ctx context.Context) ([]Discovered, error)
}
