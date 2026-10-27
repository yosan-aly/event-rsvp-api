package server

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/yosan-aly/event-rsvp-api/internal/attendance/domain"
	"github.com/yosan-aly/event-rsvp-api/internal/platform/config"
)

type stubAttendance struct{}

func (stubAttendance) Confirm(context.Context, string, int) (domain.Confirmation, error) {
	return domain.Confirmation{}, nil
}

func (stubAttendance) Update(context.Context, string, string, int) (domain.Confirmation, error) {
	return domain.Confirmation{}, nil
}

func (stubAttendance) Summary(context.Context) (domain.Summary, error) { return domain.Summary{}, nil }

func (stubAttendance) Delete(context.Context, string) error { return nil }

func TestAdminRateLimit_OnlyCountsFailedAttempts(t *testing.T) {
	cfg := config.Config{
		AllowedOrigins: "http://localhost",
		AdminToken:     "codigo-correcto",
		WriteRateLimit: 10,
		AdminRateLimit: 3,
	}
	app := New(cfg, stubAttendance{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	get := func(token string) int {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/admin/attendance-confirmations", nil)
		req.Header.Set(adminTokenHeader, token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}

	// Quien tiene el código puede consultar todas las veces que quiera.
	for i := range 10 {
		if got := get("codigo-correcto"); got != 200 {
			t.Fatalf("consulta válida %d: status %d, want 200", i+1, got)
		}
	}

	// Los intentos fallidos sí cuentan: tras 3, se bloquea.
	for i := range 3 {
		if got := get("adivinando"); got != 401 {
			t.Fatalf("intento fallido %d: status %d, want 401", i+1, got)
		}
	}
	if got := get("adivinando"); got != 429 {
		t.Fatalf("intento tras el límite: status %d, want 429", got)
	}
}
