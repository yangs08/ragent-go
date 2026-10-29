package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/term"

	"ragent-go/internal/config"
	"ragent-go/internal/database"
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
	getenv, err := config.Environment(".env", os.LookupEnv)
	if err != nil {
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
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
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer pool.Close()
	auth, err := service.NewAuthService(ctx, db, cfg.Auth)
	if err != nil {
		return err
	}
	id, err := auth.CreateUser(ctx, username, password)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"userId": id, "username": username})
}
