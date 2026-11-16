package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"version"}, {"test", "--help"}} {
		if got := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); got != 0 {
			t.Fatal(args, got)
		}
	}
	for _, args := range [][]string{{"bad"}, {"test"}, {"probe", "--host", "localhost", "--port", "0"}, {"probe", "--output", "xml"}} {
		if got := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); got != 2 {
			t.Fatal(args, got)
		}
	}
	var out bytes.Buffer
	if got := run(context.Background(), []string{"probe", "--protocol", "dns", "--host", "127.0.0.1", "--output", "json"}, &out, &bytes.Buffer{}); got != 0 {
		t.Fatal(got, out.String())
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "flows.yaml")
	os.WriteFile(p, []byte("version: 1\nflows:\n  - id: external\n    vantage: external\n    protocol: dns\n    host: localhost\n"), 0600)
	if got := run(context.Background(), []string{"test", "--config", p, "--vantage", "aks"}, &bytes.Buffer{}, &bytes.Buffer{}); got != 1 {
		t.Fatal("skips must fail the gate", got)
	}
}
