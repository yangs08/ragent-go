package bootstrap

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestBootPublishesAndCleansStartupResources(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "RAGENT_") {
			t.Setenv(key, "")
		}
	}
	t.Setenv("RAGENT_HTTP_ADDR", "127.0.0.1:9090")
	cleanup, err := Boot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if Config.Server.HTTPAddr != "127.0.0.1:9090" || Logger == nil || DB != nil {
		t.Fatal("startup resources missing")
	}
	if _, err := Boot(context.Background()); err == nil {
		t.Fatal("duplicate initialization accepted")
	}
	cleanup()
	cleanup()
	if Config.Server.HTTPAddr != "" || DB != nil {
		t.Fatal("cleanup did not release startup resources")
	}
}
