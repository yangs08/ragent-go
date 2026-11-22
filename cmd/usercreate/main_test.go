package main

import (
	"errors"
	"testing"

	"ragent-go/internal/define"
)

func TestRejectsInvalidUsernameBeforeBoot(t *testing.T) {
	t.Setenv("RAGENT_USER_PASSWORD", "valid-password")
	for _, username := range []string{"_alice", "用户abc", "alice smith", "alice@host"} {
		t.Run(username, func(t *testing.T) {
			err := run(username)
			var business *define.BusinessError
			if !errors.As(err, &business) || business.Code != define.CodeClientError || business.Message != "用户名须以字母或数字开头，且仅包含字母数字及 _.-" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
