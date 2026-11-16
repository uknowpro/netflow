package netflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepeatPreservesIntermittentFailures(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer s.Close()
	results, stats := Repeat(context.Background(), Flow{ID: "repeat", Protocol: "http", URL: s.URL}, time.Second, 3, 0, nil)
	if len(results) != 3 || stats.Completed != 3 || stats.Passed != 2 || stats.Failed != 1 || stats.NotRun != 0 || stats.Interrupted || *stats.SuccessRate < 66 || *stats.SuccessRate > 67 {
		t.Fatalf("%+v", stats)
	}
	if results[1].Code != "HTTP_STATUS_MISMATCH" || results[1].Sequence != 2 || results[1].StartedAt == nil {
		t.Fatalf("%+v", results)
	}
	if stats.TCPConnect == nil || stats.TCPConnect.Samples != 3 || stats.TCPConnect.MinMS > stats.TCPConnect.AvgMS || stats.TCPConnect.AvgMS > stats.TCPConnect.MaxMS {
		t.Fatalf("%+v", stats.TCPConnect)
	}
}
func TestRepeatCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, stats := Repeat(ctx, Flow{ID: "dns", Protocol: "dns", Host: "127.0.0.1"}, time.Second, 3, time.Hour, func(Result) { cancel() })
	if len(results) != 1 || stats.NotRun != 2 || !stats.Interrupted || stats.TCPConnect != nil {
		t.Fatalf("%+v", stats)
	}
	results, stats = Repeat(ctx, Flow{ID: "dns", Protocol: "dns", Host: "127.0.0.1"}, time.Second, 3, 0, nil)
	if len(results) != 0 || stats.SuccessRate != nil || stats.NotRun != 3 {
		t.Fatalf("%+v", stats)
	}
}
