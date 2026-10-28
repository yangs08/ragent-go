package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"
)

func TestRunServerRejectsInvalidConfig(t *testing.T) {
	err := runServer(context.Background(), func(k string) string {
		if k == "RAGENT_HTTP_ADDR" {
			return "bad"
		}
		return ""
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "RAGENT_HTTP_ADDR") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunServerReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	err = runServer(context.Background(), func(k string) string {
		if k == "RAGENT_HTTP_ADDR" {
			return listener.Addr().String()
		}
		return ""
	}, io.Discard)
	if err == nil {
		t.Fatal("expected occupied address error")
	}
}

func TestRunServerCleanShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var logs bytes.Buffer
	err = runServer(ctx, func(k string) string {
		if k == "RAGENT_HTTP_ADDR" {
			return address
		}
		return ""
	}, &logs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "server started") || !strings.Contains(logs.String(), "server stopped gracefully") {
		t.Fatalf("logs=%s", logs.String())
	}
}
