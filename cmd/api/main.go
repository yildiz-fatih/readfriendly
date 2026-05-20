package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type application struct {
	logger     *slog.Logger
	httpClient *http.Client
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		logger.Error("PORT is not set")
		os.Exit(1)
	}

	app := &application{
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	server := &http.Server{
		Addr:     ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "address", server.Addr)
	err := server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
