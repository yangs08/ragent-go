package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/term"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/service"
)

func main() {
	username := flag.String("username", "", "username to create")
	flag.Parse()
	if err := run(*username); err != nil {
		slog.Error("create user failed", "error", err)
		os.Exit(1)
	}
}
func run(username string) error {
	if username == "" {
		return errors.New("username is required; use -username")
	}
	password := os.Getenv("RAGENT_USER_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "New password: ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password from terminal failed; use RAGENT_USER_PASSWORD for automation")
		}
		password = string(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cleanup, err := bootstrap.Boot(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	if bootstrap.DB == nil {
		return errors.New("database is disabled; set RAGENT_DB_ENABLED=true")
	}

	users := service.UserService{}
	id, err := users.Create(ctx, username, password)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"userId": id, "username": username})
}
