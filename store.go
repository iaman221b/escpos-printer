package escposprinter

import (
	"context"
	"sync"
)

// SelectionStore remembers which printer an operator chose, so the choice
// survives a restart.
//
// This is an interface rather than a concrete implementation on purpose: a
// library has no business owning a database. The application already has
// storage — SQLite, a config file, a key-value store — and implements this over
// whatever it uses. Two methods is the entire contract.
//
// A nil store is valid. The registry simply does not remember selections
// between runs, and the automatic pick applies on every start.
type SelectionStore interface {
	// LoadSelectedPrinterID returns the stored printer ID, or "" if none has
	// been chosen yet. A missing value is not an error.
	LoadSelectedPrinterID(ctx context.Context) (string, error)

	// SaveSelectedPrinterID records the operator's choice.
	SaveSelectedPrinterID(ctx context.Context, id string) error
}

// MemoryStore is an in-memory SelectionStore, for tests and for applications
// that do not need the selection to outlive the process.
type MemoryStore struct {
	mu sync.RWMutex
	id string
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (m *MemoryStore) LoadSelectedPrinterID(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.id, nil
}

func (m *MemoryStore) SaveSelectedPrinterID(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.id = id
	return nil
}
