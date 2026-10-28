package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Serve drains active requests on cancellation and forcibly closes them on timeout.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler, shutdownTimeout time.Duration) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	defer server.Close()
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		// The shutdown budget must remain usable after the parent context is cancelled.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-result
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		<-result
		return nil
	}
}
