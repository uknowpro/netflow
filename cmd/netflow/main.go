package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	nf "github.com/uknowpro/netflow/internal/netflow"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(ctx context.Context, args []string, out, errs io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(errs, "netflow:", err); return 2 }
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, "netflow — network reachability from Pods and VMs\n\nUsage: netflow <test|probe|version> [flags]\nUse netflow test --help or netflow probe --help for options.")
		return 0
	}
	if args[0] == "version" {
		fmt.Fprintln(out, "netflow", version)
		return 0
	}
	if args[0] != "test" && args[0] != "probe" {
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errs)
	output := fs.String("output", "table", "Output format: table or json")
	vantage := fs.String("vantage", "", "Execution location label (does not change source IP)")
	var config, group, host, protocol, rawURL *string
	var port *int
	var timeout *time.Duration
	var retries, concurrency *int
	if args[0] == "test" {
		config = fs.String("config", "", "Path to network flows YAML (required)")
		group = fs.String("group", "", "Only run this firewall request group")
		concurrency = fs.Int("concurrency", 4, "Concurrent flows (1–64)")
	} else {
		host = fs.String("host", "", "Hostname or IP for dns/tcp/tls")
		port = fs.Int("port", 0, "Destination port for tcp/tls")
		protocol = fs.String("protocol", "tcp", "dns, tcp, tls, http, https")
		rawURL = fs.String("url", "", "Full URL for http/https")
		timeout = fs.Duration("timeout", 5*time.Second, "Timeout per attempt (up to 5m)")
		retries = fs.Int("retries", 0, "Additional attempts (0–5)")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments"))
	}
	if *output != "table" && *output != "json" {
		return fail(fmt.Errorf("output must be table or json"))
	}
	var c nf.Config
	var t time.Duration
	var err error
	workers := 1
	if args[0] == "test" {
		if *config == "" {
			return fail(fmt.Errorf("--config is required"))
		}
		workers = *concurrency
		if workers < 1 || workers > 64 {
			return fail(fmt.Errorf("concurrency must be 1–64"))
		}
		c, t, err = nf.Load(*config)
		if err != nil {
			return fail(err)
		}
		if *group != "" {
			filtered := []nf.Flow{}
			for _, f := range c.Flows {
				if f.Group == *group {
					filtered = append(filtered, f)
				}
			}
			c.Flows = filtered
		}
		if len(c.Flows) == 0 {
			return fail(fmt.Errorf("no flows match the selected group"))
		}
	} else {
		c = nf.Config{Version: 1, Defaults: nf.Defaults{Timeout: timeout.String(), Retries: *retries}, Flows: []nf.Flow{{ID: "probe", Protocol: *protocol, Host: *host, Port: *port, URL: *rawURL, Vantage: *vantage}}}
		t, err = c.Validate()
		if err != nil {
			return fail(err)
		}
	}
	source := nf.CollectSource(*vantage)
	started := time.Now().UTC()
	results := nf.Run(ctx, c.Flows, *vantage, t, c.Defaults.Retries, workers)
	summary := map[string]int{"PASS": 0, "FAIL": 0, "UNKNOWN": 0, "SKIPPED": 0}
	code := 0
	for _, r := range results {
		summary[r.Status]++
		if r.Status != "PASS" {
			code = 1
		}
	}
	if ctx.Err() != nil {
		code = 1
	}
	if *output == "json" {
		report := struct {
			SchemaVersion int            `json:"schema_version"`
			RunID         string         `json:"run_id"`
			StartedAt     time.Time      `json:"started_at"`
			Vantage       string         `json:"vantage"`
			Version       string         `json:"version"`
			Source        nf.Source      `json:"source"`
			Summary       map[string]int `json:"summary"`
			Results       []nf.Result    `json:"results"`
		}{1, started.Format("20060102T150405.000000000Z"), started, *vantage, version, source, summary, results}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fail(err)
		}
	} else {
		fmt.Fprintf(out, "Source: %s (%s/%s)  Vantage: %s\n", source.Hostname, source.OS, source.Architecture, source.Vantage)
		if k := source.Kubernetes; k != nil {
			fmt.Fprintf(out, "Pod: %s  Namespace: %s  Node: %s  Pod IP: %s (environment metadata)\n", k.PodName, k.Namespace, k.NodeName, k.PodIP)
		}
		fmt.Fprintln(out, "Source IP observed by target: UNKNOWN (NAT may change the local address)")
		for _, r := range results {
			target := r.Flow.URL
			if target == "" {
				target = r.Flow.Host
				if r.Flow.Port != 0 {
					target = fmt.Sprintf("%s:%d", target, r.Flow.Port)
				}
			}
			fmt.Fprintf(out, "\n[%s] %s  Target: %s  %dms\n", r.Status, r.Flow.ID, target, r.DurationMS)
			if r.LocalAddress != "" {
				fmt.Fprintf(out, "  Connection: %s -> %s (local socket, before NAT)\n", r.LocalAddress, r.RemoteAddress)
			} else {
				fmt.Fprintln(out, "  Connection source: UNKNOWN (no established TCP connection)")
			}
			for _, stage := range r.Stages {
				fmt.Fprintf(out, "  %-14s %-12s %-24s %dms\n", stage.Name, stage.Status, stage.Code, stage.DurationMS)
			}
			fmt.Fprintf(out, "  authentication NOT_TESTED\n  %s\n", r.Assessment.Summary)
		}
		fmt.Fprintf(out, "\nPASS=%d FAIL=%d UNKNOWN=%d SKIPPED=%d\n", summary["PASS"], summary["FAIL"], summary["UNKNOWN"], summary["SKIPPED"])
	}
	return code
}
