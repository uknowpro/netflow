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
		fmt.Fprintln(out, "netflow — network reachability from Pods and VMs\n\nUsage: netflow <test|probe|serve|version> [flags]\nUse netflow <command> --help for options.")
		return 0
	}
	if args[0] == "serve" {
		return runServe(ctx, args[1:], out, errs)
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
	var count *int
	var interval *time.Duration
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
		count = fs.Int("count", 1, "Independent samples (1–10000); count > 1 requires retries=0")
		interval = fs.Duration("interval", time.Second, "Pause between samples (0–1h)")
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
		if *count < 1 || *count > 10000 || *interval < 0 || *interval > time.Hour || (*count > 1 && *retries != 0) {
			return fail(fmt.Errorf("count must be 1–10000, interval 0–1h; repeated probes require retries=0"))
		}
		c = nf.Config{Version: 1, Defaults: nf.Defaults{Timeout: timeout.String(), Retries: *retries}, Flows: []nf.Flow{{ID: "probe", Protocol: *protocol, Host: *host, Port: *port, URL: *rawURL, Vantage: *vantage}}}
		t, err = c.Validate()
		if err != nil {
			return fail(err)
		}
	}
	source := nf.CollectSource(*vantage)
	started := time.Now().UTC()
	var results []nf.Result
	var repeat *nf.RepeatStats
	if count != nil && *count > 1 {
		var emit func(nf.Result)
		if *output == "table" {
			emit = func(r nf.Result) {
				fmt.Fprintf(out, "Sample %d/%d  %s  %s  %dms\n", r.Sequence, *count, r.Status, r.Code, r.DurationMS)
			}
		}
		samples, stats := nf.Repeat(ctx, c.Flows[0], t, *count, *interval, emit)
		results = samples
		repeat = &stats
	} else {
		results = nf.Run(ctx, c.Flows, *vantage, t, c.Defaults.Retries, workers)
	}
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
			SchemaVersion int             `json:"schema_version"`
			RunID         string          `json:"run_id"`
			StartedAt     time.Time       `json:"started_at"`
			Vantage       string          `json:"vantage"`
			Version       string          `json:"version"`
			Source        nf.Source       `json:"source"`
			Summary       map[string]int  `json:"summary"`
			Results       []nf.Result     `json:"results"`
			Repeat        *nf.RepeatStats `json:"repeat,omitempty"`
		}{1, started.Format("20060102T150405.000000000Z"), started, *vantage, version, source, summary, results, repeat}
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
			if repeat != nil && r.Sequence != repeat.Completed {
				continue
			}
			target := r.Flow.URL
			if target == "" {
				target = r.Flow.Host
				if r.Flow.Port != 0 {
					target = fmt.Sprintf("%s:%d", target, r.Flow.Port)
				}
			}
			fmt.Fprintf(out, "\n[%s] %s  Target: %s  %dms\n", r.Status, r.Flow.ID, target, r.DurationMS)
			if r.Flow.SourceEnvironment != "" {
				fmt.Fprintf(out, "  Declared source environment: %s (operator-provided)\n", r.Flow.SourceEnvironment)
			}
			if len(r.Flow.ExpectedPath) > 0 {
				fmt.Fprintln(out, "  Expected path: operator-declared; verification NOT_TESTED")
				for i, point := range r.Flow.ExpectedPath {
					fmt.Fprintf(out, "    %d. %s (%s)\n", i+1, point.Name, point.Type)
				}
			}
			if r.Flow.Protocol == "http" || r.Flow.Protocol == "https" {
				if len(r.Flow.Expect.Status) == 0 {
					fmt.Fprintln(out, "  Expected HTTP status: 200–299")
				} else {
					fmt.Fprintf(out, "  Expected HTTP status: %v\n", r.Flow.Expect.Status)
				}
				if r.HTTPStatus != 0 {
					fmt.Fprintf(out, "  Observed HTTP status: %d\n", r.HTTPStatus)
				}
			}
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
		if repeat != nil {
			rate := "N/A"
			if repeat.SuccessRate != nil {
				rate = fmt.Sprintf("%.1f%%", *repeat.SuccessRate)
			}
			fmt.Fprintf(out, "\nSamples: %d/%d  Success: %s  Not run: %d  Interrupted: %t\n", repeat.Completed, repeat.Requested, rate, repeat.NotRun, repeat.Interrupted)
			if l := repeat.TCPConnect; l != nil {
				fmt.Fprintf(out, "TCP connect latency (%d successful connections): min=%.3fms avg=%.3fms max=%.3fms\n", l.Samples, l.MinMS, l.AvgMS, l.MaxMS)
			}
		}
		fmt.Fprintf(out, "\nPASS=%d FAIL=%d UNKNOWN=%d SKIPPED=%d\n", summary["PASS"], summary["FAIL"], summary["UNKNOWN"], summary["SKIPPED"])
	}
	return code
}
