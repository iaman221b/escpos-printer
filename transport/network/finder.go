package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iaman221b/escpos-printer/device"
)

// probeTimeout bounds a single connection attempt during discovery. Short on
// purpose: a sweep opens hundreds of these, and a printer that is present
// answers a TCP handshake on the local network in milliseconds.
const probeTimeout = 300 * time.Millisecond

// probeParallelism caps concurrent dials so a subnet sweep does not exhaust
// file descriptors or trip a switch's connection limits.
const probeParallelism = 64

// Config selects how the finder looks for printers.
//
// Nothing here is read from the environment. A library must not reach into the
// host's configuration behind the caller's back — the application reads its own
// settings and passes the values in.
type Config struct {
	// Hosts are printer addresses to check directly. Each may carry a port
	// ("192.168.1.50:9100"); without one, RawPort is assumed. Exact, instant,
	// and the right answer for a shop with fixed printer addresses.
	Hosts []string

	// SweepSubnet enables a /24 sweep of this machine's own interfaces.
	//
	// Needs no configuration, but only sees printers on the same network, and
	// is off by default because sweeping a corporate network unprompted is rude
	// and can look like a port scan.
	SweepSubnet bool

	// Logger receives discovery warnings. Nil means slog.Default().
	Logger *slog.Logger
}

// Finder locates printers that have their own network address.
type Finder struct {
	cfg Config
}

// NewFinder returns a network Finder for the given configuration.
func NewFinder(cfg Config) *Finder { return &Finder{cfg: cfg} }

func (f *Finder) Name() string { return "network" }

func (f *Finder) logger() *slog.Logger {
	if f.cfg.Logger != nil {
		return f.cfg.Logger
	}
	return slog.Default()
}

func (f *Finder) Find(ctx context.Context) ([]device.Discovered, error) {
	targets := make(map[string]int)

	for _, entry := range f.cfg.Hosts {
		host, port := splitHostPort(entry)
		if host != "" {
			targets[host] = port
		}
	}

	if f.cfg.SweepSubnet {
		for _, host := range f.localSubnetHosts() {
			if _, named := targets[host]; !named {
				targets[host] = RawPort
			}
		}
	}

	if len(targets) == 0 {
		return nil, nil
	}

	return probeAll(ctx, targets), nil
}

// probeAll dials every candidate in parallel and keeps the ones that answer.
func probeAll(ctx context.Context, targets map[string]int) []device.Discovered {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		found   []device.Discovered
		limiter = make(chan struct{}, probeParallelism)
	)

	for host, port := range targets {
		wg.Add(1)
		go func(host string, port int) {
			defer wg.Done()

			limiter <- struct{}{}
			defer func() { <-limiter }()

			if !reachable(ctx, host, port) {
				return
			}

			found1 := device.Discovered{
				Device: device.Device{
					ID:         device.NetworkDeviceID(host, port),
					Name:       fmt.Sprintf("%s:%d", host, port),
					Connection: device.ConnectionNetwork,
					Transport:  "tcp",
					// A device answering on 9100 is a printer by definition —
					// nothing else uses that port — but the model is unknowable
					// over a bare TCP handshake, so it is reported as a receipt
					// printer on the strength of the port alone.
					Receipt: true,
					Detail:  "Answers on the raw ESC/POS port",
				},
				Backend: NewBackend(host, port),
			}

			mu.Lock()
			found = append(found, found1)
			mu.Unlock()
		}(host, port)
	}

	wg.Wait()
	return found
}

func reachable(ctx context.Context, host string, port int) bool {
	dialer := net.Dialer{Timeout: probeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func splitHostPort(entry string) (string, int) {
	if host, port, err := net.SplitHostPort(entry); err == nil {
		if parsed, convErr := strconv.Atoi(port); convErr == nil && parsed > 0 {
			return host, parsed
		}
		return host, RawPort
	}
	return strings.TrimSpace(entry), RawPort
}

// localSubnetHosts lists every address in the /24 around each of this
// machine's own IPv4 addresses.
//
// Limited to /24 on purpose: it is what a shop network almost always is, and it
// keeps the sweep to 254 quick connection attempts. A wider mask would mean
// tens of thousands of dials for no practical gain.
func (f *Finder) localSubnetHosts() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		f.logger().Warn("could not list network interfaces for printer discovery", "error", err)
		return nil
	}

	var hosts []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}

			base := ip.Mask(net.CIDRMask(24, 32))
			for host := 1; host < 255; host++ {
				candidate := net.IPv4(base[0], base[1], base[2], byte(host))
				if candidate.Equal(ip) {
					continue // this machine
				}
				hosts = append(hosts, candidate.String())
			}
		}
	}

	return hosts
}
