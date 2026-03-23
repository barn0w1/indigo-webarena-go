package main

import (
	"log/slog"
	"net/http"
	"os"

	indigo "github.com/barn0w1/indigo-webarena-go"
	"github.com/barn0w1/indigo-webarena-go/internal/ui/handler"
)

func main() {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client := indigo.NewClient(clientID, clientSecret,
		indigo.WithLogger(slog.Default()),
	)

	mux := http.NewServeMux()
	handler.New(client).RegisterRoutes(mux)

	slog.Info("starting server", "addr", ":"+port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
