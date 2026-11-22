package bootstrap

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestGenericValidationTags(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		valid       bool
	}{
		{"empty", "", false},
		{"short", "abc", false},
		{"12 bytes", strings.Repeat("密", 4), true},
		{"72 bytes", strings.Repeat("密", 24), true},
		{"75 bytes", strings.Repeat("密", 25), false},
		{"blank", strings.Repeat(" ", 12), false},
		{"unicode blank", strings.Repeat("　", 4), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := struct {
				Value string `binding:"required,minbytes=12,maxbytes=72,notblank"`
			}{Value: tt.value}
			err := binding.Validator.ValidateStruct(request)
			if (err == nil) != tt.valid {
				t.Fatalf("validation error: %v", err)
			}
		})
	}
}
