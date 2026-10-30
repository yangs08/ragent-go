package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ragent-go/internal/bootstrap"
	"ragent-go/internal/service"
)

const defaultPrompt = "请用一句话解释什么是 Agent。"

func main() {
	prompt := flag.String("prompt", defaultPrompt, "question for this command")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := runCheck(ctx, *prompt, os.Stdout)
	if err != nil {
		slog.Error("model check failed", "error", err)
		os.Exit(1)
	}
}

func runCheck(ctx context.Context, prompt string, output io.Writer) error {
	cleanup, err := bootstrap.Boot(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	if prompt == "" {
		prompt = defaultPrompt
	}

	if output == nil {
		return errors.New("model output is required")
	}
	client, err := service.NewModelClient(ctx, bootstrap.Config.Model)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, bootstrap.Config.Model.Timeout)
	defer cancel()
	return service.StreamModelText(ctx, client, prompt, func(text string) error {
		n, err := io.WriteString(output, text)
		if err != nil {
			return err
		}
		if n != len(text) {
			return io.ErrShortWrite
		}
		return nil
	})
}
