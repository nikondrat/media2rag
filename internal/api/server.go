package api

import (
	"context"
	"crypto/subtle"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"media2rag/internal/config"
	"media2rag/internal/dashboard"
	"media2rag/internal/embedcheck"
	"media2rag/internal/extract"
	"media2rag/internal/judge"
	"media2rag/internal/llm"
)

type Server struct {
	cfg               *config.Config
	llmClient         llm.LLMClient
	workspaceDir      string
	extractorRegistry *extract.Registry
	apiKey            string
	spaFS             fs.FS
	store             *dashboard.Store
	tracer            *dashboard.Tracer
	sse               *dashboard.SSEBroadcaster
	judgeRunner       *judge.Runner
	embedChecker      *embedcheck.Runner
}

type Options struct {
	Config            *config.Config
	LLMClient         llm.LLMClient
	WorkspaceDir      string
	ExtractorRegistry *extract.Registry
	APIKey            string
	DashboardFS       fs.FS
	Store             *dashboard.Store
	Tracer            *dashboard.Tracer
	SSE               *dashboard.SSEBroadcaster
	JudgeRunner       *judge.Runner
	EmbedChecker      *embedcheck.Runner
}

func New(opts Options) *Server {
	return &Server{
		cfg:               opts.Config,
		llmClient:         opts.LLMClient,
		workspaceDir:      opts.WorkspaceDir,
		extractorRegistry: opts.ExtractorRegistry,
		apiKey:            opts.APIKey,
		spaFS:             opts.DashboardFS,
		store:             opts.Store,
		tracer:            opts.Tracer,
		sse:               opts.SSE,
		judgeRunner:       opts.JudgeRunner,
		embedChecker:      opts.EmbedChecker,
	}
}

func (s *Server) Start(ctx context.Context, host string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/process", s.handleProcess)
	mux.HandleFunc("POST /api/query", s.handleQuery)

	mux.HandleFunc("GET /api/debug/overview", s.handleDebugOverview)
	mux.HandleFunc("GET /api/debug/timeline", s.handleDebugOverview)
	mux.HandleFunc("GET /api/debug/pipeline", s.handleDebugPipelineList)
	mux.HandleFunc("GET /api/debug/pipeline/{id}", s.handleDebugPipelineDetail)
	mux.HandleFunc("GET /api/debug/logs", s.handleDebugLogsList)
	mux.HandleFunc("GET /api/debug/logs/{id}", s.handleDebugLogDetail)
	mux.HandleFunc("GET /api/debug/metrics", s.handleDebugMetrics)
	mux.HandleFunc("GET /api/debug/documents", s.handleDebugDocuments)
	mux.HandleFunc("GET /api/debug/status", s.handleDebugStatus)
	mux.HandleFunc("GET /api/debug/config", s.handleDebugConfig)
	mux.HandleFunc("POST /api/debug/reprocess/{id}", s.handleDebugReprocess)
	mux.HandleFunc("GET /api/debug/live", s.handleDebugLiveSSE)
	mux.HandleFunc("GET /api/debug/embeddings", s.handleDebugEmbeddings)
	mux.HandleFunc("GET /api/debug/feedback", s.handleDebugFeedbackList)
	mux.HandleFunc("POST /api/debug/feedback", s.handleDebugFeedbackSubmit)
	mux.HandleFunc("GET /api/debug/regressions", s.handleDebugRegressions)
	mux.HandleFunc("GET /{path...}", s.handleDebugSPA)

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

	if s.store != nil {
		s.store.Close()
	}

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
