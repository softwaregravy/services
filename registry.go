package services

import (
	"context"
	"time"
)

// Registry is an interface implemented by types exposing a set of services.
//
// The Registry interface provides a richer query interface, it allows programs
// to resolve service names to a list of address, potentially filtering down the
// result by applying tag filtering.
//
// The interface design does come at a performance penality cost, because both
// the list of tags passed to the Lookup method and the returned list of
// addresses must be allocated on the heap. However, it is designed to only use
// standard types so code that wants to satisify the interface does not need to
// take a dependency on the package, offering strong decoupling abilities.
//
// Registry implementations must be safe to use concurrently from multiple
// goroutines.
type Registry interface {
	// Lookup returns a set of addresses at which services with the given name
	// can be reached.
	//
	// An arbitrary list of tags can be passed to the method to narrow down the
	// result set to services matching this set of tags. No tags means to do no
	// filtering.
	//
	// The method also returns a TTL representing how long the result is valid
	// for. A zero TTL means that the caller should not reuse the result.
	//
	// The returned list of addresses must not be retained by implementations of
	// the Registry interface. The caller becomes the owner of the value after
	// the method returned.
	//
	// A non-nil error is returned when the lookup cannot be completed.
	//
	// The context can be used to asynchronously cancel the query when it
	// involves blocking operations.
	Lookup(ctx context.Context, name string, tags ...string) (addrs []string, ttl time.Duration, err error)
}

// Prefer decorates the registry to prefer exposing services with tags matching
// those passed as arguments.
//
// The perference is achieved by adding the first tag to the lookup operations,
// if it returns no results the lookup is retried with the second tag, etc...
//
// If none of the lookup operation returned any results the registry falls back
// to trying without any of the preferred tags.
func Prefer(base Registry, tags ...string) Registry {
	return &prefer{
		base: base,
		tags: copyStrings(tags),
	}
}

type prefer struct {
	base Registry
	tags []string
}

func (p *prefer) Lookup(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
	tagsBuffer := make([]string, len(tags)+1)
	copy(tagsBuffer, tags)

	for _, preferredTag := range p.tags {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}

		tagsBuffer[len(tags)] = preferredTag
		addrs, ttl, err := p.base.Lookup(ctx, name, tagsBuffer...)

		if len(addrs) != 0 {
			return addrs, ttl, err
		}
	}

	return p.base.Lookup(ctx, name, tags...)
}
