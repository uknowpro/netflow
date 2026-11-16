package netflow

import (
	"context"
	"time"
)

type LatencyStats struct {
	Samples int     `json:"samples"`
	MinMS   float64 `json:"min_ms"`
	AvgMS   float64 `json:"avg_ms"`
	MaxMS   float64 `json:"max_ms"`
}
type RepeatStats struct {
	Requested   int           `json:"requested"`
	Completed   int           `json:"completed"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	NotRun      int           `json:"not_run"`
	Interrupted bool          `json:"interrupted"`
	SuccessRate *float64      `json:"success_rate_percent"`
	TCPConnect  *LatencyStats `json:"tcp_connect_latency,omitempty"`
}

// Repeat performs distinct attempts, without retries that could hide failures.
// Interval is a pause after each completed sample, not a start-to-start schedule.
func Repeat(ctx context.Context, f Flow, timeout time.Duration, count int, interval time.Duration, emit func(Result)) ([]Result, RepeatStats) {
	results := make([]Result, 0, count)
	stats := RepeatStats{Requested: count}
	var tcpTotal float64
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			break
		}
		started := time.Now().UTC()
		r := Probe(ctx, f, timeout)
		r.Sequence = i + 1
		r.StartedAt = &started
		results = append(results, r)
		stats.Completed++
		if r.Status == "PASS" {
			stats.Passed++
		} else {
			stats.Failed++
		}
		for _, s := range r.Stages {
			if s.Name != "tcp" || s.Status != "PASS" {
				continue
			}
			ms := float64(s.DurationUS) / 1000
			if stats.TCPConnect == nil {
				stats.TCPConnect = &LatencyStats{MinMS: ms, MaxMS: ms}
			}
			l := stats.TCPConnect
			l.Samples++
			tcpTotal += ms
			if ms < l.MinMS {
				l.MinMS = ms
			}
			if ms > l.MaxMS {
				l.MaxMS = ms
			}
			l.AvgMS = tcpTotal / float64(l.Samples)
		}
		if emit != nil {
			emit(r)
		}
		if i+1 < count {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	stats.NotRun = count - stats.Completed
	stats.Interrupted = ctx.Err() != nil
	if stats.Completed > 0 {
		rate := 100 * float64(stats.Passed) / float64(stats.Completed)
		stats.SuccessRate = &rate
	}
	return results, stats
}
