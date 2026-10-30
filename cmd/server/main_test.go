package main

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
)

func setupEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "RAGENT_") {
			t.Setenv(key, "")
		}
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}
func TestRunServerRejectsInvalidConfig(t *testing.T) {
	setupEnvironment(t, map[string]string{"RAGENT_HTTP_ADDR": "bad"})
	if err := runServer(context.Background()); err == nil {
		t.Fatal("invalid listener address accepted")
	}
}
func TestRunServerReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	setupEnvironment(t, map[string]string{"RAGENT_HTTP_ADDR": listener.Addr().String()})
	if err := runServer(context.Background()); err == nil {
		t.Fatal("expected occupied address error")
	}
}
func TestRunServerCleanShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	setupEnvironment(t, map[string]string{"RAGENT_HTTP_ADDR": address})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServer(ctx); err != nil {
		t.Fatal(err)
	}
}
