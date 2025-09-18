package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/MorseWayne/spike_shop/internal/config"
	"github.com/MorseWayne/spike_shop/internal/logger"
	mw "github.com/MorseWayne/spike_shop/internal/middleware"
	"github.com/MorseWayne/spike_shop/internal/resp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	// init logger
	lg, err := logger.New(cfg.App.Env, cfg.Log.Level, cfg.Log.Encoding, cfg.App.Name, cfg.App.Version)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		data := map[string]any{
			"status":  "ok",
			"version": cfg.App.Version,
		}
		resp.OK(w, &data, "", "")
	})

	// Build middleware chain: request ID -> recovery -> timeout -> CORS -> access log
	handler := mw.RequestID(mux)
	handler = mw.Recovery(lg)(handler)
	handler = mw.Timeout(cfg.App.RequestTimeout)(handler)
	handler = mw.CORS(mw.CORSConfig{
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		AllowedMethods: cfg.CORS.AllowedMethods,
		AllowedHeaders: cfg.CORS.AllowedHeaders,
	})(handler)
	handler = mw.AccessLog(lg)(handler)

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	lg.Sugar().Infow("server starting", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		lg.Sugar().Fatalw("server error", "err", err)
	}
}
