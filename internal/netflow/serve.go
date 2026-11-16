package netflow

import (
	"context"
	"errors"
	"net"
	"time"
)

type ServerEvent struct {
	SchemaVersion    int       `json:"schema_version"`
	Event            string    `json:"event"`
	Time             time.Time `json:"time"`
	LocalAddress     string    `json:"local_address"`
	PeerAddress      string    `json:"peer_address,omitempty"`
	ObservedSourceIP string    `json:"observed_source_ip,omitempty"`
	Connections      int       `json:"connections"`
	Reason           string    `json:"reason,omitempty"`
	Source           *Source   `json:"source,omitempty"`
}

// Serve accepts and immediately closes TCP connections. It never reads or echoes
// application data. Peer identity is the socket peer, which may be a proxy/NAT.
// The listener belongs to Serve and is always closed on return.
func Serve(ctx context.Context, listener net.Listener, maxConnections int, source Source, emit func(ServerEvent) error) error {
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	address := listener.Addr().String()
	event := func(kind string, count int) ServerEvent {
		return ServerEvent{SchemaVersion: 1, Event: kind, Time: time.Now().UTC(), LocalAddress: address, Connections: count}
	}
	first := event("listening", 0)
	first.Source = &source
	if err := emit(first); err != nil {
		return err
	}
	count := 0
	reason := "connection_limit"
	for maxConnections == 0 || count < maxConnections {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				reason = "canceled"
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					reason = "duration_elapsed"
				}
				break
			}
			return err
		}
		count++
		e := event("connection", count)
		e.LocalAddress = conn.LocalAddr().String()
		e.PeerAddress = conn.RemoteAddr().String()
		e.ObservedSourceIP, _, _ = net.SplitHostPort(e.PeerAddress)
		_ = conn.Close()
		if err := emit(e); err != nil {
			return err
		}
	}
	last := event("stopped", count)
	last.Reason = reason
	return emit(last)
}
