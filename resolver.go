package services

import (
	"context"
	"net"
	"strconv"
)

// The Resolver interface abstracts the concept of translating a service name
// into an address at which it can be reached.
//
// The interface is voluntarily designed to solve the simplest use case, it does
// not for the implementation to support for advanced features found in popular
// service discovery backends, which maximizes compotiblity and composability.
//
// The single method of the interface also uses basic Go types, so types that
// implement it don't even need to take a dependency on this package in order to
// satisfy the interface, making code built around this interface very flexible
// and easily decoupled of the service discovery backend being used.
type Resolver interface {
	// Lookup takes a service name as argument and returns an address at which
	// the service can be reached.
	//
	// The returned address must be a pair of an address and a port.
	// The address may be a v4 or v6 address, or a host name that can be
	// resolved by the default resolver.
	//
	// If service name resolution fails, an non-nil error is returned (but no
	// guarantee is made on the value of the address).
	//
	// The context can be used to asynchronously cancel the service name
	// resolution when it involves blocking operations.
	Lookup(ctx context.Context, name string) (addr string, err error)
}

// NewResolver returns a value implementing the Resolver interface using the
// given standard resolver.
//
// Service lookup uses LookupSRV method to resolve service names to addresses
// made of the host name where they run and the port number at which they are
// available.
//
// If r is nil, net.DefaultResolver is used.
func NewResolver(r *net.Resolver) Resolver {
	return resolver{r}
}

type resolver struct {
	*net.Resolver
}

func (r resolver) Lookup(ctx context.Context, name string) (string, error) {
	rslv := r.Resolver

	if rslv == nil {
		rslv = net.DefaultResolver
	}

	_, srv, err := rslv.LookupSRV(ctx, "", "", name)
	if err != nil {
		return "", err
	}

	host := srv[0].Target
	port := strconv.Itoa(int(srv[0].Port))
	return net.JoinHostPort(host, port), nil
}

// DefaultResolver is the default service name resolver exposed by the services
// package.
var DefaultResolver Resolver = resolver{}
