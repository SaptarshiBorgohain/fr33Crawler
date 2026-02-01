package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/knowledge-engine/backend/internal/engine"
)

type Server struct {
	Engine *engine.Engine
	Logger *logrus.Entry
	Router *http.ServeMux
}

func NewServer(eng *engine.Engine, logger *logrus.Entry) *Server {
	s := &Server{
		Engine: eng,
		Logger: logger,
		Router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.HandleFunc("/api/v1/crawl", s.handleCrawl)
	s.Router.HandleFunc("/api/v1/search", s.handleSearch)
	s.Router.HandleFunc("/api/v1/generate", s.handleGenerate)
	s.Router.HandleFunc("/api/v1/status", s.handleStatus)
}

func (s *Server) Start(port string) error {
	s.Logger.Infof("Starting API Server on %s", port)
	return http.ListenAndServe(port, s.Router)
}

// Responses
type ErrorResponse struct {
	Error string `json:"error"`
}

type SearchResponse struct {
	Query   string           `json:"query"`
	Results []SearchResultView `json:"results"`
}

type SearchResultView struct {
	URL   string  `json:"url"`
	Score float64 `json:"score"`
	Text  string  `json:"snippet"`
}

type StatusResponse struct {
	Running      bool      `json:"running"`
	PagesCrawled int64     `json:"pages_crawled"`
	Uptime       string    `json:"uptime"`
}

type GenerateResponse struct {
	Query  string `json:"query"`
	Answer string `json:"answer"`
}

// Handlers

func (s *Server) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON"})
		return
	}

	if req.URL == "" {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "URL is required"})
		return
	}

	if err := s.Engine.StartCrawl(req.URL); err != nil {
		jsonResponse(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	jsonResponse(w, http.StatusAccepted, map[string]string{"status": "crawl_started", "seed": req.URL})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "Query 'q' is required"})
		return
	}

	// Execute Search
	hits := s.Engine.VectorStore.Search(query, 5)
	
	response := SearchResponse{
		Query: query,
		Results: make([]SearchResultView, len(hits)),
	}

	for i, hit := range hits {
		txt := hit.Document.Content
		if len(txt) > 200 {
			txt = txt[:200] + "..."
		}
		response.Results[i] = SearchResultView{
			URL:   hit.Document.ID,
			Score: hit.Score,
			Text:  txt,
		}
	}

	jsonResponse(w, http.StatusOK, response)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.Engine.Stats
	
	resp := StatusResponse{
		Running:      s.Engine.IsRunning(),
		PagesCrawled: stats.PagesCrawled,
		Uptime:       time.Since(stats.StartTime).String(),
	}
	
	if !s.Engine.IsRunning() {
		resp.Uptime = "0s"
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{Error: "Query 'q' is required"})
		return
	}

	answer, err := s.Engine.GenerateAnswer(r.Context(), query)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, GenerateResponse{
		Query:  query,
		Answer: answer,
	})
}

func jsonResponse(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
