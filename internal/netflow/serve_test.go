package netflow

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestServeRecordsSocketPeerAndLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ready := make(chan struct{})
	done := make(chan error, 1)
	var events []ServerEvent
	go func() {
		done <- Serve(ctx, listener, 2, CollectSource("receiver"), func(e ServerEvent) error {
			events = append(events, e)
			if e.Event == "listening" {
				close(ready)
			}
			return nil
		})
	}()
	<-ready
	peers := []string{}
	for i := 0; i < 2; i++ {
		c, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		peers = append(peers, c.LocalAddr().String())
		c.Close()
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[3].Reason != "connection_limit" || events[3].Connections != 2 {
		t.Fatalf("%+v", events)
	}
	for i := 0; i < 2; i++ {
		if events[i+1].PeerAddress != peers[i] || events[i+1].ObservedSourceIP != "127.0.0.1" {
			t.Fatalf("%+v", events)
		}
	}
}
func TestServeStopsOnDeadlineAndOutputFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var last ServerEvent
	if err := Serve(ctx, listener, 0, Source{}, func(e ServerEvent) error { last = e; return nil }); err != nil {
		t.Fatal(err)
	}
	if last.Event != "stopped" || last.Reason != "duration_elapsed" || last.Connections != 0 {
		t.Fatalf("%+v", last)
	}
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	expected := errors.New("output failed")
	if err := Serve(context.Background(), listener, 0, Source{}, func(ServerEvent) error { return expected }); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if _, err = listener.Accept(); err == nil {
		t.Fatal("listener should be closed")
	}
}
