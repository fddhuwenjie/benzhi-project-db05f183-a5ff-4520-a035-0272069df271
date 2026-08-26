package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/httpui"
	"dialectcorpusreleasegate/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "服务失败：", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	dataDir := cfg.dataDir
	cleanup := func() {}
	if cfg.selftest {
		dataDir, err = os.MkdirTemp("", "dialect-corpus-selftest-*")
		if err != nil {
			return err
		}
		cleanup = func() { _ = os.RemoveAll(dataDir) }
	}
	defer cleanup()
	repo, err := store.Open(dataDir)
	if err != nil {
		return fmt.Errorf("打开数据目录：%w", err)
	}
	service := application.NewService(repo, 16)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := httpui.New(service, logger)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 4 * time.Second, ReadTimeout: 8 * time.Second,
		WriteTimeout: 12 * time.Second, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 1 << 20}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s：%w", cfg.addr, err)
	}
	if cfg.selftest {
		return runSelftest(server, listener, cfg.selftestTimeout)
	}
	logger.Info("方言语料放行工作台已启动", "addr", listener.Addr().String(), "data_dir", dataDir)
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
