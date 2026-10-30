package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	env "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v3"

	"ragent-go/internal/define"
)

// LoadConfig loads defaults, module YAML, then .env/system overrides. No module
// requires its own parser or manually maintained conversion map.
func LoadConfig(dir, envPath string, lookup func(string) (string, bool)) (define.Config, error) {
	values, err := LookupEnvironment(envPath, lookup)
	if err != nil {
		return define.Config{}, err
	}
	return loadConfigDir(dir, values)
}
func loadConfigDir(dir string, lookup func(string) (string, bool)) (define.Config, error) {
	var cfg define.Config
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{}}); err != nil {
		return cfg, errors.New("invalid configuration defaults")
	}
	root := reflect.ValueOf(&cfg).Elem()
	typ := root.Type()
	for i := 0; i < root.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		if err := readYAML(filepath.Join(dir, name+".yaml"), root.Field(i).Addr().Interface()); err != nil {
			return define.Config{}, err
		}
	}
	values := make(map[string]string)
	collectOverrides(root, lookup, values)
	// Defaults were applied before YAML. Do not reapply them over explicit YAML zeroes.
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: values, DefaultValueTagName: "_defaults_already_loaded"}); err != nil {
		var parseError env.ParseError
		if errors.As(err, &parseError) {
			return define.Config{}, fmt.Errorf("invalid environment configuration for field %s", parseError.Name)
		}
		return define.Config{}, errors.New("invalid environment configuration: check value types")
	}

	return cfg, nil
}
func collectOverrides(value reflect.Value, lookup func(string) (string, bool), values map[string]string) {
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		target := value.Field(i)
		key := strings.Split(field.Tag.Get("env"), ",")[0]
		if key != "" && key != "-" {
			text, present := lookup(key)
			if !present {
				continue
			}
			if text == "" {
				target.SetZero()
				text = field.Tag.Get("envDefault")
			}
			values[key] = text
		} else if target.Kind() == reflect.Struct {
			collectOverrides(target, lookup, values)
		}
	}
}
func LookupEnvironment(path string, lookup func(string) (string, bool)) (func(string) (string, bool), error) {
	values := map[string]string{}
	if path != "" {
		var err error
		values, err = godotenv.Read(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("cannot load .env: check file permissions and syntax")
		}
	}
	return func(key string) (string, bool) {
		if value, present := lookup(key); present {
			return value, true
		}
		value, present := values[key]
		return value, present
	}, nil
}
func readYAML(path string, target any) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read YAML configuration: %s", filepath.Base(path))
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil && err != io.EOF {
		return fmt.Errorf("invalid YAML configuration in %s: check field names and types; secrets belong in .env", filepath.Base(path))
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("configuration must contain a single YAML document: %s", filepath.Base(path))
	}
	return nil
}
