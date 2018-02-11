package services

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDialer(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T)
	}{
		{
			scenario: "using a dialer with a resolver that routes to a local server succeeds",
			function: testDialerWithResolver,
		},

		{
			scenario: "dialing an address that the resolver does not know about falls back to the default resolver",
			function: testDialerLocalDomain,
		},

		{
			scenario: "dialing a malformed address returns a validation error",
			function: testDialerMalformedAddress,
		},

		{
			scenario: "dialing an address where no server is listening returns an unreachable error",
			function: testDialerNoListener,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, test.function)
	}
}

func testDialerWithResolver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World!"))
	}))
	defer server.Close()

	resolver, close := dnsResolver(map[string][]string{
		"service": {server.URL[7:]},
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

	r, err := client.Get("http://service/")
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

func testDialerLocalDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World!"))
	}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.URL[7:])

	c, err := (&Dialer{}).Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Errorf("%#v", err)
	} else {
		c.Close()
	}
}

func testDialerMalformedAddress(t *testing.T) {
	_, err := (&Dialer{}).Dial("tcp", "localhost")
	if !isValidation(err) {
		t.Errorf("expected a validation error but got %#v (%s)", err, err)
	}
}

func testDialerNoListener(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	// Close the listener so connections to its address will fail.
	l.Close()

	_, err = (&Dialer{}).Dial("tcp", l.Addr().String())
	if !isUnreachable(err) {
		t.Errorf("expected an unreachable error but got %#v (%s)", err, err)
	}
}
