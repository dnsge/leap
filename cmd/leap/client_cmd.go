package main

import (
	"context"
	"fmt"
	"github.com/dnsge/leap/client"
	"github.com/dnsge/leap/client/ui"
	"github.com/urfave/cli/v2"
	"log"
	"net/http"
)

func runClient(c *cli.Context) error {
	leapClient := client.New(&client.Config{
		Domain:    c.String("domain"),
		Subdomain: c.String("subdomain"),
		LocalPort: c.Int("port"),
		Secure:    c.Bool("secure"),
	})

	var actualCtx context.Context

	if true {
		ctx, cancel := context.WithCancel(c.Context)
		u := ui.New(leapClient, ctx, cancel)
		go u.Run()
		actualCtx = ctx
	} else {
		leapClient.OnStateChange = func(state client.State) {
			log.Printf("New state: %s", state)
		}
		leapClient.OnRequest = func(r *http.Request) {
			fmt.Printf("%s %s\n%s\n", r.Method, r.URL, r.Header)
		}
		actualCtx = c.Context
	}

	return leapClient.Run(actualCtx)
}
