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

	"github.com/gin-gonic/gin/binding"
	"golang.org/x/term"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/define"
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
	request := define.CreateUserRequest{Username: username, Password: password}
	if err := binding.Validator.ValidateStruct(request); err != nil {
		return define.ClientError("用户名须为 3..64 位字母数字及 _.-；密码须为 12..72 字节且非全空白")
	}
	if !define.UsernamePattern.MatchString(request.Username) {
		return define.ClientError("用户名须以字母或数字开头，且仅包含字母数字及 _.-")
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

	id, err := service.User.Create(ctx, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"userId": id, "username": username})
}
