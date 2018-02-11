package services

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestPrefer(t *testing.T) {
	t.Run("Success", testPreferSuccess)
	t.Run("Failure", testPreferFailure)
}

func testPreferSuccess(t *testing.T) {
	registry := func(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
		if name == "my-service" {
			for _, tag := range tags {
				if tag == "B" {
					return []string{"localhost:4242"}, time.Second, nil
				}
			}
		}
		return nil, 0, nil
	}

	recorder := &lookupRecorder{
		registry: registryFunc(registry),
	}

	prefer := Prefer(recorder, "A", "B", "C")
	prefer.Lookup(context.Background(), "my-service", "my-tag")

	recorder.assertEqual(t,
		lookup{
			name: "my-service",
			tags: []string{"my-tag", "A"},
		},
		lookup{
			name:  "my-service",
			tags:  []string{"my-tag", "B"},
			addrs: []string{"localhost:4242"},
			ttl:   time.Second,
		},
	)

}

func testPreferFailure(t *testing.T) {
	registry := func(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
		return nil, 0, nil
	}

	recorder := &lookupRecorder{
		registry: registryFunc(registry),
	}

	prefer := Prefer(recorder, "A", "B", "C")
	prefer.Lookup(context.Background(), "my-service", "my-tag")

	recorder.assertEqual(t,
		lookup{
			name: "my-service",
			tags: []string{"my-tag", "A"},
		},
		lookup{
			name: "my-service",
			tags: []string{"my-tag", "B"},
		},
		lookup{
			name: "my-service",
			tags: []string{"my-tag", "C"},
		},
		lookup{
			name: "my-service",
			tags: []string{"my-tag"},
		},
	)
}

type registryFunc func(context.Context, string, ...string) ([]string, time.Duration, error)

func (f registryFunc) Lookup(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
	return f(ctx, name, tags...)
}

type lookupRecorder struct {
	registry Registry
	lookups  []lookup
}

type lookup struct {
	// input
	name string
	tags []string

	// output
	addrs []string
	ttl   time.Duration
	err   error
}

func (r *lookupRecorder) Lookup(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
	lookup := lookup{
		name: name,
		tags: copyStrings(tags),
	}

	addrs, ttl, err := r.registry.Lookup(ctx, name, tags...)

	lookup.addrs = copyStrings(addrs)
	lookup.ttl = ttl
	lookup.err = err

	r.lookups = append(r.lookups, lookup)
	return addrs, ttl, err
}

func (r *lookupRecorder) reset() {
	r.lookups = nil
}

func (r *lookupRecorder) assertEqual(t *testing.T, lookups ...lookup) {
	t.Helper()

	n1 := len(r.lookups)
	n2 := len(lookups)

	i := 0
	n := n1
	if n2 < n {
		n = n2
	}

	for i != n {
		if !reflect.DeepEqual(r.lookups[i], lookups[i]) {
			t.Error("lookups at index", i, "don't match:")
			t.Logf("- expected: %+v", lookups[i])
			t.Logf("- found:    %+v", r.lookups[i])
		}
		i++
	}

	if n1 < n2 {
		for i != n2 {
			t.Errorf("extra lookup: %+v", r.lookups[i])
			i++
		}
	}

	if n1 > n2 {
		for i != n1 {
			t.Errorf("missing lookup: %+v", lookups[i])
			i++
		}
	}
}
