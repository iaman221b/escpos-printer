package network

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/iaman221b/escpos-printer/device"
)

// RawPort is the de-facto standard "raw"/JetDirect port that nearly every
// network-attached thermal printer listens on for ESC/POS bytes.
const RawPort = 9100

// DefaultDialTimeout bounds a print connection attempt.
const DefaultDialTimeout = 3 * time.Second

// Backend prints over a raw TCP socket to the printer's ESC/POS port.
type Backend struct {
	Host        string
	Port        int
	DialTimeout time.Duration
}

// NewBackend returns a Backend for host:port. A zero port means RawPort.
func NewBackend(host string, port int) *Backend {
	if port == 0 {
		port = RawPort
	}
	return &Backend{Host: host, Port: port, DialTimeout: DefaultDialTimeout}
}

func (b *Backend) Name() string {
	return fmt.Sprintf("network:%s:%d", b.Host, b.Port)
}

func (b *Backend) addr() string {
	return net.JoinHostPort(b.Host, fmt.Sprint(b.Port))
}

func (b *Backend) dial(ctx context.Context) (net.Conn, error) {
	timeout := b.DialTimeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, "tcp", b.addr())
}

func (b *Backend) Print(ctx context.Context, data []byte) error {
	conn, err := b.dial(ctx)
	if err != nil {
		return fmt.Errorf("%w: could not reach %s: %w", device.ErrDisconnected, b.addr(), err)
	}
	defer conn.Close()

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("%w: write to %s failed: %w", device.ErrDisconnected, b.addr(), err)
	}
	return nil
}

// Status reports whether the printer answers a TCP handshake.
//
// A raw socket carries no paper signal, so paper is reported as OK and a
// genuine paper-out surfaces when a job fails. Failure to connect is returned
// as a status, not an error: "the printer is not answering" is the answer to
// the question, not a failure to ask it.
func (b *Backend) Status(ctx context.Context) (device.Status, error) {
	conn, err := b.dial(ctx)
	if err != nil {
		return device.Status{Online: false, Paper: device.PaperOut}, nil
	}
	_ = conn.Close()
	return device.Status{Online: true, Paper: device.PaperOK}, nil
}
