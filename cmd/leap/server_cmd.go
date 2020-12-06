package main

import (
	"github.com/dnsge/leap/server"
	"github.com/urfave/cli/v2"
)

func runServer(c *cli.Context) error {
	s := server.New(&server.Config{
		Domain: c.String("domain"),
		Bind:   c.String("bind"),
		Debug:  c.Bool("debug"),
	})
	return s.Run(c.Context)
}
