package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/starwalkn/gotenberg-go-client/v8"
)

type application struct {
	logger          *slog.Logger
	httpClient      *http.Client
	gotenbergClient *gotenberg.Client
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		logger.Error("PORT is not set")
		os.Exit(1)
	}

	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		logger.Error("GOTENBERG_URL is not set")
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	gotenbergClient, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := &application{
		logger:          logger,
		httpClient:      httpClient,
		gotenbergClient: gotenbergClient,
	}

	server := &http.Server{
		Addr:     ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "address", server.Addr)
	err = server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
