package escposprinter

import (
	"log/slog"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/transport/network"
)

// Option configures a Registry. Options are applied in order by New.
type Option func(*config)

type config struct {
	store           SelectionStore
	logger          *slog.Logger
	pinnedID        string
	extraFinders    []device.Finder
	replaceFinders  []device.Finder
	network         *network.Config
	skipPlatformSet bool
}

// WithStore makes the operator's printer choice survive a restart.
//
// The library never opens a database of its own; pass an implementation over
// whatever storage the application already has.
func WithStore(s SelectionStore) Option {
	return func(c *config) { c.store = s }
}

// WithLogger directs the library's diagnostics somewhere. Without it,
// slog.Default() is used.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithPin names the printer that must be active whenever it is present.
//
// A pin outranks both the remembered selection and the automatic pick: a
// terminal that states which printer it uses gets that printer, every time.
// When a pinned printer is not found, printing is disabled rather than falling
// back — see Registry.Discover for why.
//
// The value typically comes from the application's own configuration. The
// library does not read the environment itself.
func WithPin(id string) Option {
	return func(c *config) { c.pinnedID = id }
}

// WithNetwork enables discovery of printers that have their own address.
//
// Without this option no network scanning happens at all, which is the right
// default: sweeping a network unprompted is rude and can look like a port scan.
func WithNetwork(cfg network.Config) Option {
	return func(c *config) { c.network = &cfg }
}

// WithExtraFinders adds discovery sources alongside the platform defaults —
// a printer known from configuration, or hardware this library cannot find on
// its own.
func WithExtraFinders(finders ...device.Finder) Option {
	return func(c *config) { c.extraFinders = append(c.extraFinders, finders...) }
}

// WithFinders replaces the platform defaults entirely. Use it when the
// application knows exactly which sources it wants, or in tests.
func WithFinders(finders ...device.Finder) Option {
	return func(c *config) {
		c.replaceFinders = finders
		c.skipPlatformSet = true
	}
}

// finders assembles the discovery sources in the order they should report.
func (c *config) finders() []device.Finder {
	if c.skipPlatformSet {
		return c.replaceFinders
	}

	// Platform sources first — the local printer is what an operator is usually
	// looking for — then anything the caller added, then the network sweep,
	// which is the slowest.
	list := platformFinders(c.resolvedLogger())
	list = append(list, c.extraFinders...)

	if c.network != nil {
		cfg := *c.network
		if cfg.Logger == nil {
			cfg.Logger = c.resolvedLogger()
		}
		list = append(list, network.NewFinder(cfg))
	}

	return list
}

func (c *config) resolvedLogger() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}
	return slog.Default()
}
