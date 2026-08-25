package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/httpapi"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/webui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("服务退出", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.selfcheck {
		dir, err := os.MkdirTemp("", "rigging-clearance-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		cfg.dataDir = dir
	}
	store, err := eventstore.Open(cfg.dataDir)
	if err != nil {
		return err
	}
	service := application.New(store)
	handler := httpapi.New(service, webui.Handler())
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	slog.Info("舞台吊挂放行服务已启动", slog.String("addr", listener.Addr().String()), slog.Bool("selfcheck", cfg.selfcheck), slog.Uint64("eventSequence", store.Integrity().Sequence))
	if cfg.selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		checkErr := runSelfcheck(ctx, "http://"+listener.Addr().String())
		shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		shutdownErr := server.Shutdown(shutdownCtx)
		serveErr := <-errCh
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		slog.Info("完整业务自检通过")
		return nil
	}
	signalCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	select {
	case <-signalCtx.Done():
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	return server.Shutdown(shutdownCtx)
}
