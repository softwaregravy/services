package services

import (
	"context"
	"net"
	"time"
)

// A Dialer contains options for connecting to an address.
//
// Dialer differs from the standard library's dialer by using a custom resolver
// to translate service names into network addresses.
//
// The zero value for each field is equivalent to dialing without that option.
// Dialing with the zero value of Dialer is therefore equivalent to just calling
// the Dial function.
type Dialer struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. If Deadline is also set, it may fail
	// earlier.
	//
	// The default is no timeout.
	//
	// When using TCP and dialing a host name with multiple IP
	// addresses, the timeout may be divided between them.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	Timeout time.Duration

	// Deadline is the absolute point in time after which dials
	// will fail. If Timeout is set, it may fail earlier.
	// Zero means no deadline, or dependent on the operating system
	// as with the Timeout option.
	Deadline time.Time

	// LocalAddr is the local address to use when dialing an
	// address. The address must be of a compatible type for the
	// network being dialed.
	// If nil, a local address is automatically chosen.
	LocalAddr net.Addr

	// DualStack enables RFC 6555-compliant "Happy Eyeballs"
	// dialing when the network is "tcp" and the host in the
	// address parameter resolves to both IPv4 and IPv6 addresses.
	// This allows a client to tolerate networks where one address
	// family is silently broken.
	DualStack bool

	// FallbackDelay specifies the length of time to wait before
	// spawning a fallback connection, when DualStack is enabled.
	// If zero, a default delay of 300ms is used.
	FallbackDelay time.Duration

	// KeepAlive specifies the keep-alive period for an active
	// network connection.
	// If zero, keep-alives are not enabled. Network protocols
	// that do not support keep-alives ignore this field.
	KeepAlive time.Duration

	// Resolver optionally specifies an alternate resolver to use.
	Resolver Resolver
}

// Dial connects to the address on the named network.
//
// See https://golang.org/pkg/net/#Dialer.Dial for more details,
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the address on the named network using the provided
// context.
//
// See https://golang.org/pkg/net/#Dialer.DialContext for more details.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)

	if err != nil || net.ParseIP(host) == nil {
		resolver := d.Resolver

		if resolver == nil {
			resolver = DefaultResolver
		}

		target, err := resolver.Resolve(ctx, nameOnly(address))
		switch {
		case err == nil:
			address = target
		case isUnreachable(err):
		default:
			return nil, err
		}
	}

	dialer := net.Dialer{
		Timeout:       d.Timeout,
		Deadline:      d.Deadline,
		LocalAddr:     d.LocalAddr,
		DualStack:     d.DualStack,
		FallbackDelay: d.FallbackDelay,
		KeepAlive:     d.KeepAlive,
	}

	conn, err := dialer.DialContext(ctx, network, address)
	return conn, wrapError(err)
}

func nameOnly(address string) string {
	name, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return name
}
