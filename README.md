# services  [![CircleCI](https://circleci.com/gh/segmentio/services.svg?style=shield)](https://circleci.com/gh/segmentio/services) [![Go Report Card](https://goreportcard.com/badge/github.com/segmentio/services)](https://goreportcard.com/report/github.com/segmentio/services) [![GoDoc](https://godoc.org/github.com/segmentio/services?status.svg)](https://godoc.org/github.com/segmentio/services)

*Go package providing building blocks to work with service discovery patterns.*

## Motivations

When building large and dynamically scaling infrastructures made of complex
pieces of software, classic ways of mapping service names to addresses at which
they are available don't fit anymore. We need ways to indentify services that
may be coming and going, binding random ports, registering to service discovery
backends. As systems get more and more complex, they also end up needed more
control over service name resolution to implement sharding solutions or custom
load balancing algorithms.

This is what the *services* package attempts to help with. It provides building
blocks for abstracting and integrating programs into more advanced service
discovery models, in a way that integrates with the Go standard library.

## Resolver

One of the core concepts of the *services* package is the
[Resolver](https://godoc.org/github.com/segmentio/services#Resolver).
This interface abstracts the concept of translating a service name into an
address at which the service can be reached.

The package offers a default implementation of this interface which makes DNS
queries of the SRV type to obtain both the target host where the service is
running, and the port at which it can be reached.

This example acts like a mini-version of `dig <service> SRV`:
```go
package main

import (
    "fmt"
    "os"

    "github.com/segmentio/services"
)

func main() {
    r := services.NewResolver(nil)

    a, err := r.Resolve(ctx, os.Args[1])
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    fmt.Println(a)
}
```

## Dialer

While service name resolution is at the core of every service discovery system,
it is mostly useful to establish connections to running services.

The *services* package exposes a [Dialer](https://godoc.org/github.com/segmentio/services#Dialer)
type which mirrors exactly the API exposed by the standard `net.Dialer`, but
uses a Resolver to lookup addresses in order to establish connections.

Matching the standard library's design allows the Dialer to be injected in
places where dial functions are expected. For example, this code modifies the
default HTTP transport to use a service dialer for establishing all new TCP
connections:

```go
package main

import (
    "net/http"

    "github.com/segmentio/services"
)

func init() {
    if t, ok := http.DefaultTransport.(*http.Transport); ok {
        t.DialContext = (&services.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
                DualStack: true,
        }).DialContext
    }
}

...
```
