package main

import (
	"context"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func signalInterrupterContext() context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			cancel()
		}()
		<-c
	}()

	return ctx
}

func main() {
	ctx := signalInterrupterContext()

	app := &cli.App{
		Name:  "leap",
		Usage: "A tunnel to your local environment for HTTP requests",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug mode",
				Value: false,
				Hidden: true,
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "expose",
				Usage:  "expose your local environment",
				Action: runClient,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domain",
						Aliases:  []string{"d"},
						Usage:    "Domain of the leap server",
						EnvVars:  []string{"LEAP_DOMAIN"},
						Required: true,
					},
					&cli.StringFlag{
						Name:        "subdomain",
						Aliases:     []string{"s"},
						Usage:       "Requested subdomain to tunnel through",
						EnvVars:     []string{"LEAP_SUBDOMAIN"},
						DefaultText: "random",
					},
					&cli.IntFlag{
						Name:     "port",
						Aliases:  []string{"p"},
						Usage:    "Local port to expose",
						EnvVars:  []string{"LEAP_PORT"},
						Required: true,
						Value:    80,
					},
					&cli.BoolFlag{
						Name:        "secure",
						Aliases:     nil,
						Usage:       "Whether the leap server is HTTPS",
						Value:       true,
						DefaultText: "true",
					},
				},
			},
			{
				Name:   "host",
				Usage:  "host a leap server",
				Action: runServer,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domain",
						Aliases:  []string{"d"},
						Usage:    "Domain of the leap server",
						EnvVars:  []string{"LEAP_DOMAIN"},
						Required: true,
					},
					&cli.StringFlag{
						Name:    "bind",
						Aliases: []string{"b"},
						Usage:   "Address to bind the leap server to",
						EnvVars: []string{"LEAP_BIND"},
						Value:   "0.0.0.0:8080",
					},
				},
			},
		},
	}

	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
