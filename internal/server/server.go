package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/tour"
)

// Server serves the tour web UI and REST API.
type Server struct {
	port         int
	repoDir      string
	model        string
	session      *session.Session
	clotildeRoot string
	tours        map[string]*tour.Tour
}

// New creates a new Server.
func New(port int, repoDir string, model string, sess *session.Session, clotildeRoot string) *Server {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		absRepoDir = filepath.Clean(repoDir)
	}
	// Resolve symlinks so containment checks in fileContent are reliable
	if real, err := filepath.EvalSymlinks(absRepoDir); err == nil {
		absRepoDir = real
	}
	return &Server{
		port:         port,
		repoDir:      absRepoDir,
		model:        model,
		session:      sess,
		clotildeRoot: clotildeRoot,
	}
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	s.loadTours()

	static := staticHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", s.sessionInfo)
	mux.HandleFunc("GET /api/tours", s.tourList)
	mux.HandleFunc("GET /api/tours/{name}", s.tourDetail)
	mux.HandleFunc("GET /api/files/{path...}", s.fileContent)
	mux.HandleFunc("GET /api/tree", s.fileTree)
	mux.HandleFunc("GET /ws/chat", s.chatHandler)
	mux.Handle("GET /static/", http.StripPrefix("/static/", static))
	mux.HandleFunc("GET /{$}", s.serveIndex)
	return logRequests(mux)
}

// Start loads tours and starts the HTTP server on localhost.
func (s *Server) Start() error {
	handler := s.Handler()
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	fmt.Fprintf(os.Stderr, "Tour server listening on http://%s\n", listener.Addr())
	return http.Serve(listener, handler)
}

func (s *Server) loadTours() {
	toursDir := filepath.Join(s.repoDir, ".tours")
	tours, err := tour.LoadFromDir(toursDir)
	if err != nil {
		s.tours = make(map[string]*tour.Tour)
		return
	}
	s.tours = tours
}

func (s *Server) sessionInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data := map[string]string{
		"name": s.session.Name,
		"id":   s.session.Metadata.SessionID,
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) tourList(w http.ResponseWriter, _ *http.Request) {
	type tourSummary struct {
		Name  string `json:"name"`
		Title string `json:"title"`
		Steps int    `json:"steps"`
	}

	var list []tourSummary
	for name, t := range s.tours {
		list = append(list, tourSummary{
			Name:  name,
			Title: t.Title,
			Steps: len(t.Steps),
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	writeJSON(w, http.StatusOK, list)
}

func (s *Server) tourDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	t, ok := s.tours[name]
	if !ok {
		http.Error(w, "tour not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) fileContent(w http.ResponseWriter, r *http.Request) {
	relPath := r.PathValue("path")
	if relPath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	absPath, err := filepath.Abs(filepath.Join(s.repoDir, filepath.FromSlash(relPath)))
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Resolve symlinks to prevent escaping the repo via symlinks inside it
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Ensure the resolved path is within the repo directory
	rel, err := filepath.Rel(s.repoDir, realPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Block excluded directories at any depth (same policy as /api/tree)
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if excludeDirs[segment] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

// excludeDirs lists directories to skip in the file tree.
var excludeDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	".tours":       true,
	"dist":         true,
	"vendor":       true,
}

func (s *Server) fileTree(w http.ResponseWriter, _ *http.Request) {
	var files []string
	err := filepath.WalkDir(s.repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible entries, continue walk
		}

		rel, err := filepath.Rel(s.repoDir, path)
		if err != nil {
			return nil //nolint:nilerr // skip entries with path errors, continue walk
		}

		if d.IsDir() {
			if excludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		http.Error(w, "failed to walk directory", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, files)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "%s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
