package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
)

type Server struct {
	mux      *http.ServeMux
	binary   string
}

func NewServer() (*Server, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable: %w", err)
	}

	s := &Server{
		mux:    http.NewServeMux(),
		binary: binary,
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/openapi.json", s.handleOpenAPI)
	s.mux.HandleFunc("/process", s.handleProcess)
	s.mux.HandleFunc("/rag", s.handleRAG)
	s.mux.HandleFunc("/index", s.handleIndex)
	s.mux.HandleFunc("/documents", s.handleDocuments)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/preprocess", s.handlePreprocess)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("api/openapi.json")
	if err != nil {
		http.Error(w, "openapi.json not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type processRequest struct {
	Source    string `json:"source"`
	OutputDir string `json:"output_dir,omitempty"`
	Force     bool   `json:"force,omitempty"`
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req processRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Source == "" {
		http.Error(w, "source is required", http.StatusBadRequest)
		return
	}

	args := []string{"process", req.Source}
	if req.OutputDir != "" {
		args = append(args, "-o", req.OutputDir)
	}
	if req.Force {
		args = append(args, "--force")
	}

	s.runCommand(w, args)
}

type ragRequest struct {
	Query string `json:"query"`
	Top   int    `json:"top,omitempty"`
}

func (s *Server) handleRAG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ragRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	args := []string{"rag", req.Query}
	if req.Top > 0 {
		args = append(args, "--top", fmt.Sprintf("%d", req.Top))
	}

	s.runCommand(w, args)
}

type indexRequest struct {
	Directory string `json:"directory"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req indexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Directory == "" {
		http.Error(w, "directory is required", http.StatusBadRequest)
		return
	}

	s.runCommand(w, []string{"index", req.Directory})
}

type documentsRequest struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
}

func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req documentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Action == "" {
		http.Error(w, "action is required", http.StatusBadRequest)
		return
	}

	args := []string{"documents", req.Action}
	if req.ID != "" {
		args = append(args, req.ID)
	}

	s.runCommand(w, args)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runCommand(w, []string{"health"})
}

type preprocessRequest struct {
	Query    string `json:"query"`
	Variants int    `json:"variants,omitempty"`
}

func (s *Server) handlePreprocess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req preprocessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	args := []string{"preprocess", req.Query}
	if req.Variants > 1 {
		args = append(args, "--variants", fmt.Sprintf("%d", req.Variants))
	}

	s.runCommand(w, args)
}

func (s *Server) runCommand(w http.ResponseWriter, args []string) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, s.binary, args...)
	output, err := cmd.CombinedOutput()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error: %s\n%s", err, string(output))
		return
	}

	w.Write(output)
}

func (s *Server) Start(addr string) error {
	fmt.Printf("OpenAPI server starting on %s\n", fmt.Sprintf("http://localhost%s", addr[1:]))
	fmt.Printf("OpenAPI spec: http://localhost%s/openapi.json\n", addr[1:])
	return http.ListenAndServe(addr, s)
}
