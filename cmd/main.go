package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	attendanceapp "github.com/yosan-aly/event-rsvp-api/internal/attendance/application"
	attendancepg "github.com/yosan-aly/event-rsvp-api/internal/attendance/infrastructure/postgres"
	"github.com/yosan-aly/event-rsvp-api/internal/platform/config"
	"github.com/yosan-aly/event-rsvp-api/internal/platform/database"
	"github.com/yosan-aly/event-rsvp-api/internal/platform/logger"
	"github.com/yosan-aly/event-rsvp-api/internal/platform/server"
)

// @title Event RSVP API
// @version 1.0.0
// @description Confirmación de asistencia de los invitados a un evento y listado
// @description con totales para los organizadores (protegido con token de administración).
// @host localhost:7070
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey AdminToken
// @in header
// @name X-Admin-Token
// @description Token de administración (variable ADMIN_TOKEN)
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	// Solo en local: .env (secretos, ignorado por git) y luego .dev.env.
	// godotenv no sobrescribe variables ya definidas, así que en Cloud Run
	// (donde estos archivos no existen) mandan las del entorno.
	_ = godotenv.Load(".env")
	_ = godotenv.Load(".dev.env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	// Cloud Run envía SIGTERM antes de apagar la instancia.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool, cfg.DB.Schema); err != nil {
		return err
	}

	svc := attendanceapp.NewService(attendancepg.NewRepository(pool))
	app := server.New(cfg, svc, log)

	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(":" + cfg.Port) }()
	log.Info("server started", "port", cfg.Port, "adminEnabled", cfg.AdminToken != "")

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
