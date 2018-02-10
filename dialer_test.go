package services

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestDialer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World!"))
	}))
	defer server.Close()

	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	portNumber, _ := strconv.Atoi(port)

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
			Port:     uint16(portNumber),
			Target:   "localhost.",
		})
		w.WriteMsg(a)
	})
	defer s.Shutdown()

	c := &http.Client{
		Transport: &http.Transport{
			DialContext: (&Dialer{
				Resolver: NewResolver(&net.Resolver{
					PreferGo: true,
					Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, s.Net, s.Addr)
					},
				}),
			}).DialContext,
		},
	}

	r, err := c.Get("http://service/")
	if err != nil {
		t.Error(err)
		return
	}
	defer r.Body.Close()

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}

	if string(b) != "Hello World!" {
		t.Error("http response body mismatch:", string(b))
	}
}
