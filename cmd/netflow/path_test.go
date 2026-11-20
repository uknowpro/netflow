package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	nf "github.com/uknowpro/netflow/internal/netflow"
)

func TestDeclaredPathAndHTTPOutcome(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(403) }))
	defer server.Close()
	for _, tc := range []struct {
		name, vantage string
		status, code  int
	}{
		{"unexpected-response", "aks", 200, 1},
		{"expected-response", "aks", 403, 0},
		{"wrong-location", "vm", 200, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "flows.yaml")
			cfg := fmt.Sprintf("version: 1\nflows:\n  - id: web\n    vantage: aks\n    source_environment: aks\n    protocol: http\n    url: %s\n    expect:\n      status: [%d]\n    expected_path:\n      - type: external_waf\n        name: edge\n      - type: application_gateway\n        name: internal\n", server.URL, tc.status)
			if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"json", "table"} {
				before := hits.Load()
				var out, errs bytes.Buffer
				code := run(context.Background(), []string{"test", "--config", path, "--vantage", tc.vantage, "--output", format}, &out, &errs)
				if code != tc.code {
					t.Fatalf("exit %d: %s", code, errs.String())
				}
				if tc.vantage == "vm" && hits.Load() != before {
					t.Fatal("skipped flow sent traffic")
				}
				if format == "json" {
					var report struct {
						Results []nf.Result `json:"results"`
					}
					if err := json.Unmarshal(out.Bytes(), &report); err != nil {
						t.Fatal(err)
					}
					got := report.Results[0]
					if got.Assessment.ExpectedPathVerification != "NOT_TESTED" || got.Assessment.FirewallVerdict != "UNKNOWN" || len(got.Flow.ExpectedPath) != 2 || got.Flow.Expect.Status[0] != tc.status || got.Flow.SourceEnvironment != "aks" {
						t.Fatalf("%+v", got)
					}
					if tc.vantage == "aks" && (got.HTTPStatus != 403 || got.Assessment.TCPConnectivity != "PASS") {
						t.Fatalf("%+v", got)
					}
				} else if !strings.Contains(out.String(), "operator-declared; verification NOT_TESTED") || !strings.Contains(out.String(), "Expected HTTP status:") {
					t.Fatal(out.String())
				}
			}
		})
	}
}

func TestEnvironmentExamples(t *testing.T) {
	for _, name := range []string{"aks", "azure-vm", "onprem-vm"} {
		cfg, _, err := nf.Load("../../examples/" + name + "-flows.yaml")
		if err != nil {
			t.Fatal(err)
		}
		for _, flow := range cfg.Flows {
			if flow.Vantage != name || len(flow.ExpectedPath) == 0 {
				t.Fatal(flow)
			}
		}
	}
}
