package api

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"media2rag/internal/config"
	"media2rag/internal/extract"
	"media2rag/internal/llm"
)

type Server struct {
	cfg               *config.Config
	llmClient         llm.LLMClient
	workspaceDir      string
	extractorRegistry *extract.Registry
	apiKey            string
}

type Options struct {
	Config            *config.Config
	LLMClient         llm.LLMClient
	WorkspaceDir      string
	ExtractorRegistry *extract.Registry
	APIKey            string
}

func New(opts Options) *Server {
	return &Server{
		cfg:               opts.Config,
		llmClient:         opts.LLMClient,
		workspaceDir:      opts.WorkspaceDir,
		extractorRegistry: opts.ExtractorRegistry,
		apiKey:            opts.APIKey,
	}
}

func (s *Server) Start(ctx context.Context, host string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/process", s.handleProcess)
	mux.HandleFunc("POST /api/query", s.handleQuery)

	var handler http.Handler = mux
	handler = recoveryMiddleware(handler)
	handler = loggingMiddleware(handler)
	handler = corsMiddleware(handler)

	if s.apiKey != "" {
		handler = s.apiKeyMiddleware(handler)
	}

	addr := host + ":" + strconv.Itoa(port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("HTTP server starting on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	select {
	case <-quit:
		log.Println("shutting down server...")
	case <-ctx.Done():
		log.Println("context cancelled, shutting down...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(s.apiKey)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
