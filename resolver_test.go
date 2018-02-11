package services

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestResolver(t *testing.T) {
	testResolver(t, dnsResolver)
}

type newResolverFunc func(map[string][]string) (r Resolver, close func())

// testResolver is a test suite to validate that Resolver implementations behave
// the same.
//
// The function takes a constructor of Resolvers which is given as parameter a
// map of service names to possible address lists.
func testResolver(t *testing.T, newResolver newResolverFunc) {
	t.Helper()

	tests := []struct {
		scenario string
		function func(*testing.T, newResolverFunc)
	}{
		{
			scenario: "calling Resolve with a context that was canceled returns a canceled error",
			function: testResolverCancel,
		},

		{
			scenario: "calling Resolve with a valid service name returns one of the service addresses",
			function: testResolverSuccess,
		},

		{
			scenario: "calling Resolve with an unknown service name returns an unreachable error",
			function: testResolverFailure,
		},

		{
			scenario: "using the resolver in the context of an HTTP request routes connections to the correct servers",
			function: testResolverWithHTTP,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) { test.function(t, newResolver) })
	}
}

func testResolverCancel(t *testing.T, newResolver newResolverFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolver, close := newResolver(nil)
	defer close()

	_, err := resolver.Resolve(ctx, "my-service")
	if !isCanceled(err) {
		t.Errorf("expected a canceled error but got %#v (%s)", err, err)
	}
}

func testResolverSuccess(t *testing.T, newResolver newResolverFunc) {
	services := map[string][]string{
		"service-1": {
			"localhost:4000",
			"localhost:4001",
			"localhost:4002",
			"localhost:4003",
		},
		"service-2": {
			"localhost:4004",
			"localhost:4005",
		},
		"service-3": {
			"localhost:4005",
			"localhost:4007",
			"localhost:4008",
		},
	}

	resolver, close := newResolver(services)
	defer close()

	for i := 0; i != 10; i++ {
		for name, addrs := range services {
			a, err := resolver.Resolve(context.Background(), name)
			if err != nil {
				t.Errorf("%#v (%s)", err, err)
				return
			}

			ok := false
			for _, addr := range addrs {
				if addr == a {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("resolving %s: %s is not one of the services address %s", name, a, addrs)
			}
		}
	}
}

func testResolverFailure(t *testing.T, newResolver newResolverFunc) {
	resolver, close := newResolver(nil)
	defer close()

	for i := 0; i != 10; i++ {
		_, err := resolver.Resolve(context.Background(), "whatever")
		if !isUnreachable(err) {
			t.Error("expected an unreachable error but got %#v (%s)", err, err)
		}
	}
}

func testResolverWithHTTP(t *testing.T, newResolver newResolverFunc) {
	handler1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1"))
	})

	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2"))
	})

	// service-1
	server10 := httptest.NewServer(handler1)
	defer server10.Close()

	server11 := httptest.NewServer(handler1)
	defer server11.Close()

	server12 := httptest.NewServer(handler1)
	defer server12.Close()

	// service-2
	server20 := httptest.NewServer(handler2)
	defer server20.Close()

	server21 := httptest.NewServer(handler2)
	defer server21.Close()

	server22 := httptest.NewServer(handler2)
	defer server22.Close()

	resolver, close := newResolver(map[string][]string{
		"service-1": {
			server10.URL[7:],
			server11.URL[7:],
			server12.URL[7:],
		},
		"service-2": {
			server20.URL[7:],
			server21.URL[7:],
			server22.URL[7:],
		},
	})
	defer close()

	transport := &http.Transport{
		DialContext: (&Dialer{
			Resolver: resolver,
		}).DialContext,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
	}

	for i := 0; i != 10; i++ {
		for i, service := range []string{"service-1", "service-2"} {
			r, err := client.Get("http://" + service + "/")
			if err != nil {
				t.Errorf("GET http://%s/: %#v (%s)", service, err, err)
				continue
			}
			defer r.Body.Close()

			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			if string(b) != strconv.Itoa(i+1) {
				t.Errorf("GET http://%s/: http response body mismatch: %s", service, string(b))
			}
		}
	}
}

func BenchmarkResolver(b *testing.B) {
	benchmarkResolver(b, dnsResolver)
}

// benchmarkResolver implements a benchmark suite for impementations of the
// Resolver interface.
func benchmarkResolver(b *testing.B, newResolver newResolverFunc) {
	b.Helper()

	resolver, close := newResolver(map[string][]string{
		"service-1": {
			"localhost:4000",
			"localhost:4001",
			"localhost:4002",
			"localhost:4003",
		},
		"service-2": {
			"localhost:4004",
			"localhost:4005",
		},
		"service-3": {
			"localhost:4005",
			"localhost:4007",
			"localhost:4008",
		},
	})
	defer close()

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resolver.Resolve(ctx, "service-1")
		}
	})
}

func dnsResolver(services map[string][]string) (r Resolver, close func()) {
	server := dnsServer(func(w dns.ResponseWriter, r *dns.Msg) {
		a := &dns.Msg{}
		a.SetReply(r)
		a.Authoritative = true
		a.RecursionAvailable = true

		qname := strings.TrimSuffix(r.Question[0].Name, ".")
		qtype := r.Question[0].Qtype

		switch qtype {
		case dns.TypeA:
			if qname == "localhost" {
				a.Answer = append(a.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    10,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}

		case dns.TypeSRV:
			if candidates, ok := services[qname]; ok {
				service := candidates[rand.Intn(len(candidates))]
				_, port, _ := net.SplitHostPort(service)
				portNumber, _ := strconv.Atoi(port)

				a.Answer = append(a.Answer, &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    10,
					},
					Priority: 1,
					Weight:   1,
					Port:     uint16(portNumber),
					Target:   "localhost.",
				})
			}
		}

		if len(a.Answer) == 0 {
			a.Rcode = dns.RcodeNameError
		}

		w.WriteMsg(a)
	})

	resolver := NewResolver(&net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, server.Net, server.Addr)
		},
	})

	return resolver, func() { server.Shutdown() }
}

func dnsServer(handler func(dns.ResponseWriter, *dns.Msg)) *dns.Server {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	s := &dns.Server{
		Net:        c.LocalAddr().Network(),
		Addr:       c.LocalAddr().String(),
		PacketConn: c,
		Handler:    dns.HandlerFunc(handler),
	}

	go s.ActivateAndServe()
	return s
}
