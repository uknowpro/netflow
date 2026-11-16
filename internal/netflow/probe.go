package netflow

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Stage struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Code       string `json:"code,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}
type Result struct {
	Flow          Flow       `json:"flow"`
	Status        string     `json:"status"`
	Code          string     `json:"code,omitempty"`
	Attempts      int        `json:"attempts"`
	DurationMS    int64      `json:"duration_ms"`
	ResolvedIPs   []string   `json:"resolved_ips,omitempty"`
	LocalAddress  string     `json:"local_address,omitempty"`
	RemoteAddress string     `json:"remote_address,omitempty"`
	HTTPStatus    int        `json:"http_status,omitempty"`
	Stages        []Stage    `json:"stages"`
	FailedStage   string     `json:"failed_stage,omitempty"`
	Assessment    Assessment `json:"assessment"`
}

func Probe(ctx context.Context, f Flow, timeout time.Duration) Result {
	return probe(ctx, f, timeout, nil)
}

func probe(ctx context.Context, f Flow, timeout time.Duration, roots *x509.CertPool) (r Result) {
	started := time.Now()
	r = Result{Flow: f, Status: "PASS", Attempts: 1, Stages: []Stage{}}
	defer r.finalize()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	add := func(name string, start time.Time, err error, code string) bool {
		s := Stage{Name: name, Status: "PASS", DurationMS: time.Since(start).Milliseconds()}
		if err != nil {
			s.Status = "FAIL"
			s.Code = code
			r.Status = "FAIL"
			r.Code = code
		}
		r.Stages = append(r.Stages, s)
		r.DurationMS = time.Since(started).Milliseconds()
		return err == nil
	}
	host, port := f.Host, f.Port
	if f.URL != "" {
		u, _ := url.Parse(f.URL)
		host = u.Hostname()
		port = 80
		if f.Protocol == "https" {
			port = 443
		}
		if u.Port() != "" {
			port, _ = strconv.Atoi(u.Port())
		}
	}
	start := time.Now()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err == nil && len(ips) == 0 {
		err = fmt.Errorf("no addresses")
	}
	if !add("dns", start, err, classify(err, "DNS_ERROR")) {
		return r
	}
	for _, ip := range ips {
		r.ResolvedIPs = append(r.ResolvedIPs, ip.String())
	}
	if f.Protocol == "dns" {
		return r
	}
	// Use the resolved address list so the recorded DNS result is the one tested.
	start = time.Now()
	var conn net.Conn
	for i, ip := range ips {
		remaining := timeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining = time.Until(deadline)
		}
		attemptCtx, stop := context.WithTimeout(ctx, remaining/time.Duration(len(ips)-i))
		conn, err = (&net.Dialer{}).DialContext(attemptCtx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
		stop()
		if err == nil {
			break
		}
	}
	if !add("tcp", start, err, classify(err, "CONNECT_ERROR")) {
		return r
	}
	defer conn.Close()
	r.LocalAddress = conn.LocalAddr().String()
	r.RemoteAddress = conn.RemoteAddr().String()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	if f.Protocol == "tcp" {
		return r
	}
	var secure *tls.Conn
	if f.Protocol == "tls" || f.Protocol == "https" {
		start = time.Now()
		secure = tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, RootCAs: roots})
		err = secure.HandshakeContext(ctx)
		if !add("tls", start, err, classify(err, "TLS_ERROR")) {
			return r
		}
	}
	if f.Protocol == "tls" {
		return r
	}
	start = time.Now()
	// Direct connections, no proxy or redirects: test only the configured destination.
	tr := &http.Transport{DisableKeepAlives: true, DialContext: func(context.Context, string, string) (net.Conn, error) { return conn, nil }, DialTLSContext: func(context.Context, string, string) (net.Conn, error) { return secure, nil }}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		add("http", start, err, "HTTP_ERROR")
		return r
	}
	resp, err := client.Do(req)
	if err != nil {
		add("http", start, err, classify(err, "HTTP_ERROR"))
		return r
	}
	defer resp.Body.Close()
	r.HTTPStatus = resp.StatusCode
	matched := resp.StatusCode >= 200 && resp.StatusCode <= 299
	if len(f.Expect.Status) > 0 {
		matched = false
		for _, s := range f.Expect.Status {
			if s == resp.StatusCode {
				matched = true
			}
		}
	}
	if !matched {
		err = fmt.Errorf("unexpected status")
	}
	add("http", start, err, "HTTP_STATUS_MISMATCH")
	return r
}
func classify(err error, fallback string) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELED"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "CONNECTION_REFUSED"
	}
	var n net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &n) && n.Timeout()) {
		if fallback == "CONNECT_ERROR" {
			return "CONNECT_TIMEOUT"
		}
		return "TIMEOUT"
	}
	return fallback
}
func Run(ctx context.Context, flows []Flow, vantage string, timeout time.Duration, retries, concurrency int) []Result {
	out := make([]Result, len(flows))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				f := flows[i]
				if f.Vantage != "" && f.Vantage != vantage {
					out[i] = Result{Flow: f, Status: "SKIPPED", Code: "VANTAGE_MISMATCH", Stages: []Stage{}}
					out[i].finalize()
					continue
				}
				start := time.Now()
				for a := 0; a <= retries; a++ {
					out[i] = Probe(ctx, f, timeout)
					out[i].Attempts = a + 1
					if out[i].Status == "PASS" || ctx.Err() != nil {
						break
					}
					if a < retries {
						select {
						case <-ctx.Done():
						case <-time.After(200 * time.Millisecond):
						}
					}
				}
				out[i].DurationMS = time.Since(start).Milliseconds()
			}
		}()
	}
	for i := range flows {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}
