package bootstrap

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }), time.Second)
	}()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	if conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		conn.Close()
		t.Fatal("listener still accepts connections")
	}
}

func TestServeListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	if err := Serve(context.Background(), listener, http.NotFoundHandler(), time.Second); err == nil {
		t.Fatal("expected listener error")
	}
}

func TestServeShutdownTimeoutClosesActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	exited := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(exited) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, handler, 20*time.Millisecond) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := &http.Client{Timeout: time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown hung")
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("active handler was not cancelled")
	}
	<-clientDone
}
