package escposprinter

import (
	"context"
	"strings"

	"github.com/iaman221b/escpos-printer/device"
	"github.com/iaman221b/escpos-printer/transport/network"
)

// ConfiguredDevice names a printer explicitly, instead of relying on discovery
// to turn it up.
//
// Use it when the application already knows which printer this terminal uses.
// The named printer is always offered and is pinned active, so an operator's
// stated choice beats anything found by looking around — and it keeps working
// when a transport would otherwise hide the device, such as a Windows queue
// that is temporarily offline and therefore skipped during discovery.
//
// The values typically come from the application's own settings. This library
// does not read the environment itself.
type ConfiguredDevice struct {
	// Mode selects the transport:
	//
	//   "windows", "spooler"   a Windows print queue      (Windows only)
	//   "cups"                 a CUPS queue               (Linux/macOS only)
	//   "usb", "device"        a USB device node path     (Linux/macOS only)
	//   "network"              a printer with its own address
	//
	// A mode that does not apply to the running platform is ignored rather than
	// rejected, so one shared configuration can serve a mixed fleet without the
	// other platforms refusing to start.
	Mode string

	// Name is the queue name, or the device node path for "usb"/"device".
	Name string

	// Host and Port apply to Mode "network". A zero Port means network.RawPort.
	Host string
	Port int
}

// WithConfiguredDevice adds an explicitly named printer to discovery and pins
// it active.
//
// Pinning is the point: see Registry.Discover for what happens when a pinned
// printer is not present.
func WithConfiguredDevice(cfg ConfiguredDevice) Option {
	return func(c *config) {
		finder := &configuredFinder{cfg: cfg}
		if id := finder.pinnedID(); id != "" {
			c.pinnedID = id
		}
		// Prepended rather than appended so the configured device is reported
		// first, ahead of whatever discovery turns up.
		c.extraFinders = append([]device.Finder{finder}, c.extraFinders...)
	}
}

// configuredFinder yields the explicitly named printer, or nothing.
type configuredFinder struct {
	cfg ConfiguredDevice
}

func (c *configuredFinder) Name() string { return "configured" }

func (c *configuredFinder) Find(ctx context.Context) ([]device.Discovered, error) {
	found, ok := c.resolve()
	if !ok {
		return nil, nil
	}
	return []device.Discovered{found}, nil
}

// pinnedID is the ID of the configured printer, or "" when none applies here.
func (c *configuredFinder) pinnedID() string {
	found, ok := c.resolve()
	if !ok {
		return ""
	}
	return found.Device.ID
}

func (c *configuredFinder) resolve() (device.Discovered, bool) {
	mode := strings.ToLower(strings.TrimSpace(c.cfg.Mode))
	name := strings.TrimSpace(c.cfg.Name)

	switch mode {
	case "":
		return device.Discovered{}, false

	case "network":
		host := strings.TrimSpace(c.cfg.Host)
		if host == "" {
			return device.Discovered{}, false
		}
		port := c.cfg.Port
		if port <= 0 {
			port = network.RawPort
		}
		return device.Discovered{
			Device: device.Device{
				ID:         device.NetworkDeviceID(host, port),
				Name:       host,
				Connection: device.ConnectionNetwork,
				Transport:  "tcp",
				Receipt:    true,
				Detail:     "Named in the application's configuration",
			},
			Backend: network.NewBackend(host, port),
		}, true

	default:
		// A local queue, whose handling differs per platform. See
		// configuredLocalDevice in platform_windows.go and platform_unix.go.
		if name == "" {
			return device.Discovered{}, false
		}
		return configuredLocalDevice(mode, name)
	}
}
