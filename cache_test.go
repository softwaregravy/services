package services

import (
	"context"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	tests := []struct {
		scenario string
		newCache func(Registry) *Cache
	}{
		{
			scenario: "small cache",
			newCache: func(r Registry) *Cache {
				return &Cache{
					Registry: r,
					MaxBytes: 64,
				}
			},
		},

		{
			scenario: "large cache",
			newCache: func(r Registry) *Cache {
				return &Cache{
					Registry: r,
					MaxBytes: 1024 * 1024,
				}
			},
		},

		{
			scenario: "short TTL",
			newCache: func(r Registry) *Cache {
				return &Cache{
					Registry: r,
					MaxTTL:   1 * time.Nanosecond,
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			t.Run("resolver", func(t *testing.T) {
				testResolver(t, func(services map[string][]string) (Resolver, func()) {
					cache := test.newCache(registry(services))

					close := func() {
						stats := cache.Stats()
						t.Logf("%+v", cache.Stats())

						if (stats.Evictions + stats.Size) != stats.Misses {
							t.Error("the number of cache misses does not match the sum of the size and evictions")
						}
					}

					return cache, close
				})
			})
		})
	}
}

func BenchmarkCache(b *testing.B) {
	benchmarkResolver(b, func(services map[string][]string) (Resolver, func()) {
		return &Cache{Registry: registry(services)}, func() {}
	})
}

type registry map[string][]string

func (r registry) Lookup(ctx context.Context, name string, tags ...string) ([]string, time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	addrs, ok := r[name]
	if !ok {
		return nil, time.Second, unreachable{}
	}

	return copyStrings(addrs), time.Second, nil
}

type unreachable struct{}

func (unreachable) Error() string     { return "unreachable" }
func (unreachable) Unreachable() bool { return true }
