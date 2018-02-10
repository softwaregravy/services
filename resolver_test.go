package services

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestResolver(t *testing.T) {
	s := dnsServer(func(w dns.ResponseWriter, r *dns.Msg) {
		a := &dns.Msg{}
		a.SetReply(r)
		a.Authoritative = true
		a.RecursionAvailable = true
		a.Answer = append(a.Answer, &dns.SRV{
			Hdr: dns.RR_Header{
				Name:   r.Question[0].Name,
				Rrtype: dns.TypeSRV,
				Class:  dns.ClassINET,
				Ttl:    10,
			},
			Priority: 1,
			Weight:   1,
			Port:     4242,
			Target:   "host.services.local.",
		})
		w.WriteMsg(a)
	})
	defer s.Shutdown()

	r := NewResolver(&net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, s.Net, s.Addr)
		},
	})

	for i := 0; i != 10; i++ {
		addr, err := r.Resolve(context.Background(), "whatever")
		if err != nil {
			t.Error(err)
		}
		if addr != "host.services.local:4242" {
			t.Error("unexpected address returned:", addr)
		}
	}
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
