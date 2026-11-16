package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"time"

	nf "github.com/uknowpro/netflow/internal/netflow"
)

func runServe(ctx context.Context, args []string, out, errs io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errs)
	address := fs.String("listen", "127.0.0.1:8080", "TCP bind address; use 0.0.0.0:8080 or [::]:8080 for remote access")
	duration := fs.Duration("duration", 10*time.Minute, "Stop after this duration (greater than zero, up to 24h)")
	limit := fs.Int("max-connections", 0, "Stop after N accepted connections; 0 means no count limit")
	output := fs.String("output", "table", "Output format: table or json (JSON Lines)")
	vantage := fs.String("vantage", "", "Execution location label")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(errs, "netflow serve:", err); return 2 }
	if fs.NArg() != 0 || *duration <= 0 || *duration > 24*time.Hour || *limit < 0 || (*output != "table" && *output != "json") {
		return fail(fmt.Errorf("invalid arguments: check --help"))
	}
	serverCtx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(serverCtx, "tcp", *address)
	if err != nil {
		return fail(err)
	}
	enc := json.NewEncoder(out)
	emit := func(e nf.ServerEvent) error {
		if *output == "json" {
			return enc.Encode(e)
		}
		var err error
		switch e.Event {
		case "listening":
			_, err = fmt.Fprintf(out, "Listening: %s (TCP accept-only)\nSource: %s (%s/%s)\n", e.LocalAddress, e.Source.Hostname, e.Source.OS, e.Source.Architecture)
		case "connection":
			_, err = fmt.Fprintf(out, "%s ACCEPT #%d peer=%s local=%s observed_source_ip=%s\n", e.Time.Format(time.RFC3339Nano), e.Connections, e.PeerAddress, e.LocalAddress, e.ObservedSourceIP)
		case "stopped":
			_, err = fmt.Fprintf(out, "Stopped: %s; accepted=%d\n", e.Reason, e.Connections)
		}
		return err
	}
	if err := nf.Serve(serverCtx, listener, *limit, nf.CollectSource(*vantage), emit); err != nil {
		return fail(err)
	}
	return 0
}
