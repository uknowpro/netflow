package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestRepeatCLI(t *testing.T) {
	var out bytes.Buffer
	args := []string{"probe", "--protocol", "dns", "--host", "127.0.0.1", "--count", "3", "--interval", "0s", "--output", "json"}
	if code := run(context.Background(), args, &out, &bytes.Buffer{}); code != 0 {
		t.Fatal(code, out.String())
	}
	var report struct {
		Repeat struct {
			Completed int `json:"completed"`
			Passed    int `json:"passed"`
		} `json:"repeat"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Repeat.Completed != 3 || report.Repeat.Passed != 3 || len(report.Results) != 3 {
		t.Fatal(out.String())
	}
	for _, extra := range [][]string{{"--count", "0"}, {"--interval", "-1s"}, {"--retries", "1"}} {
		if code := run(context.Background(), append(append([]string{}, args...), extra...), &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
			t.Fatal(extra, code)
		}
	}
}
func TestServeCLI(t *testing.T) {
	var out bytes.Buffer
	if code := run(context.Background(), []string{"serve", "--listen", "127.0.0.1:0", "--duration", "100ms", "--output", "json"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatal(code, out.String())
	}
	dec := json.NewDecoder(&out)
	var first, last struct {
		Event  string `json:"event"`
		Reason string `json:"reason"`
	}
	if err := dec.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&last); err != nil {
		t.Fatal(err)
	}
	if first.Event != "listening" || last.Reason != "duration_elapsed" {
		t.Fatal(first, last)
	}
	for _, args := range [][]string{{"serve", "--duration", "0s"}, {"serve", "--max-connections", "-1"}, {"serve", "--output", "csv"}, {"serve", "--listen", "not-an-address"}} {
		if code := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
			t.Fatal(args, code)
		}
	}
}
