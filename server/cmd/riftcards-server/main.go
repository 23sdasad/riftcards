package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mjq/riftcards/server/internal/match"
	"github.com/mjq/riftcards/server/internal/metrics"
	"github.com/mjq/riftcards/server/internal/transport/ws"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	logLevel := flag.String("log-level", "info", "debug, info, warn, or error")
	// 固定种子让对局可复现，仅供本地端到端测试使用；生产必须保持 0（每局随机）。
	seed := flag.Int64("seed", 0, "fixed match seed for local tests; 0 means random per match (test-only)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(*logLevel),
	}))
	slog.SetDefault(logger)

	registry := metrics.New()
	manager := match.NewManager(logger, registry, *seed)
	wsHandler := ws.NewHandler(manager, logger, []string{"localhost:*", "127.0.0.1:*"}, registry)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /metrics", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_ = registry.WriteText(writer)
	})
	mux.Handle("GET /ws", wsHandler)

	// 先显式监听再服务：绑定失败要立刻失败，并记录解析后的真实地址
	// （`-addr 127.0.0.1:0` 因此可以被测试用于自动选端口）。
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("listen failed", "addr", *addr, "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening",
			"addr", listener.Addr().String(),
			"wsPath", "/ws",
			"metricsPath", "/metrics",
			"fixedSeed", *seed,
		)
		errCh <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
