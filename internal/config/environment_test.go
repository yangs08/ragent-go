package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentFileAndPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	data := "# local configuration\nFILE_ONLY='hello # world'\nOVERRIDE=file\nEMPTY_OVERRIDE=file\nEQUALS=\"a=b\"\nBLANK=\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"OVERRIDE": "system", "EMPTY_OVERRIDE": ""}
	getenv, err := Environment(path, func(key string) (string, bool) { v, ok := values[key]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"FILE_ONLY": "hello # world", "OVERRIDE": "system", "EMPTY_OVERRIDE": "", "EQUALS": "a=b", "BLANK": "", "MISSING": ""} {
		if got := getenv(key); got != want {
			t.Errorf("%s=%q want %q", key, got, want)
		}
	}
	if _, exists := os.LookupEnv("FILE_ONLY"); exists {
		t.Fatal("file changed process environment")
	}
}

func TestEnvironmentMissingFile(t *testing.T) {
	getenv, err := Environment(filepath.Join(t.TempDir(), ".env"), func(key string) (string, bool) { return "system", true })
	if err != nil {
		t.Fatal(err)
	}
	if getenv("KEY") != "system" {
		t.Fatal("system environment unavailable")
	}
}

func TestEnvironmentRejectsBrokenFileWithoutLeakingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("KEY='private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Environment(path, os.LookupEnv)
	if err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatalf("unsafe error: %v", err)
	}
	_, err = Environment(t.TempDir(), os.LookupEnv)
	if err == nil {
		t.Fatal("directory must not be accepted as .env file")
	}
}
