package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// Environment reads a dotenv file without modifying process-wide environment.
// Explicit system values, including empty strings, take precedence over the file.
func Environment(path string, lookup func(string) (string, bool)) (func(string) string, error) {
	values, err := godotenv.Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Parser errors may include file contents; expose only a safe diagnostic.
		return nil, errors.New("cannot load .env: check file permissions and syntax")
	}
	return func(key string) string {
		if value, exists := lookup(key); exists {
			return value
		}
		return values[key]
	}, nil
}
