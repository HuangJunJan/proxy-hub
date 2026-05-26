package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/monitor"
	"proxy-hub/internal/server"
	"proxy-hub/internal/store"
)

func main() {
	os.Exit(run())
}

func run() int {
	var configPath string
	var host string
	var port int
	var noBrowser bool
	flag.StringVar(&configPath, "config", defaultConfigPath(), "path to config.yaml")
	flag.StringVar(&host, "host", "", "override listen host")
	flag.IntVar(&port, "port", 0, "override listen port")
	flag.BoolVar(&noBrowser, "no-browser", false, "disable automatic browser launch")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	cfgMgr := config.NewManager(configPath, logger)
	if err := cfgMgr.Load(); err != nil {
		logger.Error("failed to load config", "path", configPath, "error", err)
		return 1
	}
	if err := ensureAdminPasswordHash(cfgMgr); err != nil {
		logger.Error("failed to normalize admin password hash", "path", configPath, "error", err)
		return 1
	}
	cfg := cfgMgr.Snapshot()
	listenHost := cfg.EffectiveServerHost()
	if host != "" {
		listenHost = host
	}
	listenPort := cfg.EffectiveServerPort()
	if port != 0 {
		listenPort = port
	}
	sessions, err := auth.NewSessionManager()
	if err != nil {
		logger.Error("failed to initialize sessions", "error", err)
		return 1
	}
	dbPath := filepath.Join(filepath.Dir(configPath), "proxy-hub.db")
	storeDB, err := store.OpenSQLite(context.Background(), dbPath, logger)
	if err != nil {
		logger.Error("failed to open sqlite store", "path", dbPath, "error", err)
		return 1
	}
	defer storeDB.Close()

	monitorCtx, stopMonitor := context.WithCancel(context.Background())
	defer stopMonitor()
	monitorService := monitor.NewService(storeDB, storeDB, logger, monitor.Options{})
	go monitorService.Run(monitorCtx, monitor.Options{})
	go monitorService.RunCleanup(monitorCtx, func() int {
		return cfgMgr.Snapshot().EffectiveRequestLogRetentionDays()
	})
	watchCtx, stopWatch := context.WithCancel(context.Background())
	defer stopWatch()
	go func() {
		if err := cfgMgr.Watch(watchCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("config watcher stopped", "error", err)
		}
	}()

	addr := listenHost + ":" + strconv.Itoa(listenPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.NewRouter(server.Options{Logger: logger, ConfigManager: cfgMgr, Sessions: sessions, Monitor: monitorService, Logs: storeDB, Stats: storeDB}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("proxy hub listening", "addr", addr, "config", configPath, "setup_needed", cfgMgr.SetupNeeded())
		if cfg.EffectiveOpenBrowser() && !noBrowser {
			go openBrowser(logger, browserURL(listenHost, listenPort))
		}
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stop:
		logger.Info("shutdown requested", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			logger.Error("server failed", "error", err)
			return 1
		}
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stopWatch()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		return 1
	}
	stopMonitor()
	return 0
}

func ensureAdminPasswordHash(cfgMgr *config.Manager) error {
	cfg := cfgMgr.Snapshot()
	if cfg.Admin == nil || cfg.Admin.PasswordHash == "" || auth.IsArgon2IDHash(cfg.Admin.PasswordHash) {
		return nil
	}
	plaintext := cfg.Admin.PasswordHash
	hashed, err := auth.HashPassword(plaintext)
	if err != nil {
		return err
	}
	return cfgMgr.Save(func(next *config.Config) error {
		if next.Admin != nil && next.Admin.PasswordHash == plaintext {
			next.Admin.PasswordHash = hashed
		}
		return nil
	})
}

func defaultConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}

func browserURL(host string, port int) string {
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func openBrowser(logger *slog.Logger, url string) {
	time.Sleep(200 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		logger.Warn("failed to open browser", "url", url, "error", err)
	}
}
