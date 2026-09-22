package proxy

import (
	"context"
	"errors"
	"testing"
)

func TestExtractServiceNameFromPath(t *testing.T) {
	tests := []struct {
		path   string
		suffix string
		want   string
	}{
		{"/api/services/web/start", "/start", "web"},
		{"/api/services/web/stop", "/stop", "web"},
		{"/api/services/web/settings", "/settings", "web"},
		{"/api/services/web", "", "web"},
		{"/api/services/web", "/", "web"},
		{"/api/services/", "/start", ""},
		{"/api/services/my-service/start", "/start", "my-service"},
		{"/api/services/web/start/extra", "/start", "web/start/extra"},
	}
	for _, tt := range tests {
		got := extractServiceNameFromPath(tt.path, tt.suffix)
		if got != tt.want {
			t.Errorf("extractServiceNameFromPath(%q, %q) = %q, want %q", tt.path, tt.suffix, got, tt.want)
		}
	}
}

func TestErrorsIsCtx(t *testing.T) {
	t.Run("context.Canceled", func(t *testing.T) {
		if !errorsIsCtx(context.Canceled) {
			t.Error("errorsIsCtx(context.Canceled) should be true")
		}
	})

	t.Run("context.DeadlineExceeded", func(t *testing.T) {
		if !errorsIsCtx(context.DeadlineExceeded) {
			t.Error("errorsIsCtx(context.DeadlineExceeded) should be true")
		}
	})

	t.Run("wrapped context.Canceled", func(t *testing.T) {
		err := errors.Join(errors.New("op failed"), context.Canceled)
		if !errorsIsCtx(err) {
			t.Error("errorsIsCtx(wrapped context.Canceled) should be true")
		}
	})

	t.Run("wrapped DeadlineExceeded", func(t *testing.T) {
		err := errors.Join(errors.New("timeout"), context.DeadlineExceeded)
		if !errorsIsCtx(err) {
			t.Error("errorsIsCtx(wrapped DeadlineExceeded) should be true")
		}
	})

	t.Run("plain error", func(t *testing.T) {
		if errorsIsCtx(errors.New("some error")) {
			t.Error("errorsIsCtx(plain error) should be false")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if errorsIsCtx(nil) {
			t.Error("errorsIsCtx(nil) should be false")
		}
	})
}
