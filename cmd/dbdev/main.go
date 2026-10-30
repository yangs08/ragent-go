// dbdev passes the resolved YAML/env database settings to local Compose.
// It deliberately loads configuration without opening a database connection.
package main

import (
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"ragent-go/internal/bootstrap"
)

func main() {
	command := flag.String("command", "up", "up or stop")
	flag.Parse()
	if err := run(*command); err != nil {
		slog.Error("local database command failed", "error", err)
		os.Exit(1)
	}
}
func run(command string) error {
	if command != "up" && command != "stop" {
		return errors.New("supported local database commands: up, stop")
	}
	cfg, err := bootstrap.LoadConfig("configs", ".env", os.LookupEnv)
	if err != nil {
		return err
	}
	db := cfg.Database
	args := []string{"compose", "--env-file", ".env", "-f", "deploy/compose.yaml"}
	if command == "up" {
		args = append(args, "up", "-d", "--wait", "postgres")
	} else {
		args = append(args, "stop", "postgres")
	}
	child := exec.Command("docker", args...)
	values := map[string]string{"RAGENT_DB_NAME": db.Name, "RAGENT_DB_USER": db.User, "RAGENT_DB_PASSWORD": db.Password, "RAGENT_DB_PORT": strconv.Itoa(db.Port)}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := values[key]; !replace {
			child.Env = append(child.Env, item)
		}
	}
	for key, value := range values {
		child.Env = append(child.Env, key+"="+value)
	}
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Run(); err != nil {
		return errors.New("docker compose failed; verify Docker and local database settings")
	}
	return nil
}
