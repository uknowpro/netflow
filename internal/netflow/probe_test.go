package netflow

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTCPAndDNS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	f := Flow{ID: "tcp", Protocol: "tcp", Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}
	r := Probe(context.Background(), f, time.Second)
	if r.Status != "PASS" || r.LocalAddress == "" || len(r.Stages) != 2 {
		t.Fatalf("%+v", r)
	}
	ln.Close()
	r = Probe(context.Background(), f, time.Second)
	if r.Status != "FAIL" || r.Code != "CONNECTION_REFUSED" {
		t.Fatalf("%+v", r)
	}
	f.Protocol = "dns"
	f.Port = 0
	r = Probe(context.Background(), f, time.Second)
	if r.Status != "PASS" || len(r.ResolvedIPs) == 0 {
		t.Fatalf("%+v", r)
	}
}
func TestHTTPStagesAndNoRedirect(t *testing.T) {
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.WriteHeader(200) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer server.Close()
	f := Flow{ID: "http", Protocol: "http", URL: server.URL}
	r := Probe(context.Background(), f, time.Second)
	if r.Code != "HTTP_STATUS_MISMATCH" || r.HTTPStatus != 302 || r.Stages[1].Status != "PASS" || hits != 0 {
		t.Fatalf("%+v hits=%d", r, hits)
	}
	f.Expect.Status = []int{302}
	r = Probe(context.Background(), f, time.Second)
	if r.Status != "PASS" {
		t.Fatalf("%+v", r)
	}
}
func TestUntrustedTLS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()
	r := Probe(context.Background(), Flow{ID: "tls", Protocol: "https", URL: s.URL}, 5*time.Second)
	if r.Code != "TLS_ERROR" || r.Stages[1].Status != "PASS" {
		t.Fatalf("%+v", r)
	}
}
func TestTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	r := Probe(context.Background(), Flow{ID: "slow", Protocol: "http", URL: s.URL}, 50*time.Millisecond)
	if r.Status != "FAIL" || r.Code != "TIMEOUT" {
		t.Fatalf("%+v", r)
	}
}
func TestRunVantageRetriesOrder(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer s.Close()
	flows := []Flow{{ID: "skip", Vantage: "external", Protocol: "http", URL: s.URL}, {ID: "fail", Vantage: "aks", Protocol: "http", URL: s.URL}}
	r := Run(context.Background(), flows, "aks", time.Second, 1, 2)
	if r[0].Status != "SKIPPED" || r[0].Attempts != 0 || r[1].Attempts != 2 || r[1].Status != "FAIL" {
		t.Fatalf("%+v", r)
	}
}
func TestConfigStrict(t *testing.T) {
	valid := "version: 1\nflows:\n  - id: db\n    protocol: tcp\n    host: localhost\n    port: 5432\n"
	for _, tc := range []struct {
		name, body string
		bad        bool
	}{
		{"valid", valid, false}, {"unknown", valid + "typo: true\n", true}, {"multi", valid + "---\nversion: 1\n", true}, {"duplicate", valid + "  - id: db\n    protocol: dns\n    host: localhost\n", true}, {"udp", strings.ReplaceAll(valid, "tcp", "udp"), true}, {"invalid-port", strings.ReplaceAll(valid, "5432", "0"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "flows.yaml")
			if err := os.WriteFile(p, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, _, err := Load(p)
			if (err != nil) != tc.bad {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTrustedHTTPS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	roots := x509.NewCertPool()
	roots.AddCert(s.Certificate())
	r := probe(context.Background(), Flow{ID: "https", Protocol: "https", URL: s.URL}, 5*time.Second, roots)
	if r.Status != "PASS" || r.HTTPStatus != 200 || len(r.Stages) != 4 {
		t.Fatalf("%+v", r)
	}
	u, _ := url.Parse(s.URL)
	port, _ := strconv.Atoi(u.Port())
	r = probe(context.Background(), Flow{ID: "tls", Protocol: "tls", Host: u.Hostname(), Port: port}, 5*time.Second, roots)
	if r.Status != "PASS" || len(r.Stages) != 3 {
		t.Fatalf("%+v", r)
	}
}

func TestAssessmentDoesNotClaimAuthentication(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer s.Close()
	r := Probe(context.Background(), Flow{ID: "auth", Protocol: "http", URL: s.URL}, time.Second)
	if r.Assessment.TCPConnectivity != "PASS" || r.Assessment.Authentication != "NOT_TESTED" || r.Assessment.FirewallVerdict != "UNKNOWN" || r.FailedStage != "http" {
		t.Fatalf("%+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = Probe(ctx, Flow{ID: "cancel", Protocol: "https", URL: s.URL}, time.Second)
	if r.Status != "FAIL" || r.Stages[len(r.Stages)-1].Status != "NOT_TESTED" {
		t.Fatalf("%+v", r)
	}
}

func TestContextOwnsClosedSocketDiagnosis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := classifyStage(ctx, net.ErrClosed, "HTTP_ERROR"); got != "CANCELED" {
		t.Fatal(got)
	}
	ctx, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	if got := classifyStage(ctx, net.ErrClosed, "HTTP_ERROR"); got != "TIMEOUT" {
		t.Fatal(got)
	}
	if got := classifyStage(ctx, net.ErrClosed, "CONNECT_ERROR"); got != "CONNECT_TIMEOUT" {
		t.Fatal(got)
	}
}
