package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
	"vortex/internal/balancer"
	"vortex/internal/config"
	"vortex/internal/core"
	"vortex/internal/runtime"
	"vortex/internal/vortex"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

var vortexInstance atomic.Pointer[vortex.Vortex]

func main() {
	initialLogger := setupLogger(config.LogLevelInfo)

	vtx, cfg, logger := buildVortex(initialLogger)
	if vtx == nil {
		initialLogger.Fatal().Msg("failed to build initial vortex instance")
	}
	vortexInstance.Store(vtx)

	handler := panicRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vortexInstance.Load().ServeRequest(w, r)
	}), logger)
	server := http.Server{
		Addr:    cfg.Server.Listen,
		Handler: handler,
	}

	adminServer := startAdminServer(cfg.Admin.Listen, logger)

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("unable to start vortex")
		}
	}()
	logger.Info().Msgf("vortex is listening on %s", server.Addr)

	<-shutdownCh
	logger.Info().Msg("shutdown signal received, exiting gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if adminServer != nil {
		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("failed to gracefully shut down the admin server")
		}
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("failed to gracefully shut down the main server")
	}

	logger.Info().Msg("servers have been gracefully shut down")
}

func setupLogger(logLevel config.LogLevel) zerolog.Logger {
	appEnv := os.Getenv("VORTEX_APP_ENVIRONMENT")

	var zeroLogLevel zerolog.Level
	switch logLevel {
	case config.LogLevelDebug:
		zeroLogLevel = zerolog.DebugLevel
	case config.LogLevelError:
		zeroLogLevel = zerolog.ErrorLevel
	default:
		zeroLogLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(zeroLogLevel)

	var logger zerolog.Logger
	if appEnv == "dev" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout})
		zerolog.TimeFieldFormat = time.RFC3339
	} else {
		logger = zerolog.New(os.Stdout)
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	}

	logger = logger.With().
		Timestamp().
		Logger()

	return logger
}

func startAdminServer(listenAddr string, logger zerolog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./web/dashboard.html")
	})

	mux.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if reload(logger) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Configuration reloaded successfully.\n"))
		} else {
			http.Error(w, "Failed to reload configuration", http.StatusInternalServerError)
		}
	})

	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		logger.Info().Msgf("admin server is listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("admin server failed")
		}
	}()

	return server
}

func buildVortex(logger zerolog.Logger) (*vortex.Vortex, *config.Config, zerolog.Logger) {
	cfgPath := os.Getenv("VORTEX_CONFIG_FILE_PATH")
	if cfgPath == "" {
		cfgPath = "."
	}
	cfgName := os.Getenv("VORTEX_CONFIG_FILE_NAME")
	if cfgName == "" {
		cfgName = "vortex"
	}

	cfg, err := config.LoadConfig(cfgPath, cfgName)
	if err != nil {
		logger.Error().Err(err).Msg("error loading config")
		return nil, nil, zerolog.Nop()
	}

	internalCfg, err := core.BuildInternalConfig(cfg)
	if err != nil {
		logger.Error().Err(err).Msg("error building internal config")
		return nil, nil, zerolog.Nop()
	}

	runtimeState := runtime.NewRuntimeState(internalCfg)
	vortexLogger := setupLogger(cfg.Logging.Level)
	blc := balancer.NewVortexBalancer(runtimeState)
	vtx := vortex.NewVortex(internalCfg, runtimeState, blc, vortexLogger)

	return vtx, cfg, vortexLogger
}

func reload(logger zerolog.Logger) bool {
	logger.Info().Msg("reload signal received, attempting to reload...")

	newVortex, _, _ := buildVortex(logger)
	if newVortex != nil {
		vortexInstance.Store(newVortex)
		logger.Info().Msg("configuration reloaded successfully")
		return true
	}

	logger.Error().Msg("failed to reload configuration, continuing with the old one")
	return false
}

// TODO: сделать чистый мэйн
