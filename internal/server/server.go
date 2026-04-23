package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	fsops "github.com/warriorguo/websz/internal/fs"
)

type Config struct {
	Root     string
	Token    string
	ReadOnly bool
}

type Server struct {
	config *Config
	fm     *fsops.FileManager
	mux    *http.ServeMux
}

func NewServer(config *Config, staticFiles embed.FS) (*Server, error) {
	fm, err := fsops.NewFileManager(config.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to create file manager: %w", err)
	}

	s := &Server{
		config: config,
		fm:     fm,
		mux:    http.NewServeMux(),
	}

	s.setupRoutes(staticFiles)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := corsMiddleware(loggingMiddleware(s.mux))
	
	// Apply authentication middleware if token is required
	if s.config.Token != "" && !s.shouldBypassAuth(r.URL.Path) {
		if !s.isAuthenticated(r) {
			// For API endpoints, return JSON error
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/open") {
				writeError(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			// For other endpoints, redirect to auth page
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}
	}
	
	handler.ServeHTTP(w, r)
}

func (s *Server) setupRoutes(staticFiles embed.FS) {
	// Auth endpoint
	s.mux.HandleFunc("/auth", s.handleAuth)
	
	// API endpoints
	s.mux.HandleFunc("/api/list", s.handleList)
	s.mux.HandleFunc("/api/find", s.handleFind)
	s.mux.HandleFunc("/api/stat", s.handleStat)
	s.mux.HandleFunc("/api/download", s.handleDownload)
	s.mux.HandleFunc("/open", s.handleOpen)
	
	if !s.config.ReadOnly {
		s.mux.HandleFunc("/api/upload", s.handleUpload)
		s.mux.HandleFunc("/api/put", s.handlePut)
		s.mux.HandleFunc("/api/mkdir", s.handleMkdir)
		s.mux.HandleFunc("/api/rename", s.handleRename)
		s.mux.HandleFunc("/api/delete", s.handleDelete)
	}

	subFS, err := fs.Sub(staticFiles, "dist")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(subFS)))
	} else {
		fmt.Printf("Error creating subFS: %v\n", err)
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>websz</title></head>
<body>
<h1>websz File Manager</h1>
<p>Frontend not built yet. Use API endpoints directly:</p>
<ul>
<li><a href="/api/list?p=/">List files</a></li>
</ul>
</body>
</html>`))
				return
			}
			http.NotFound(w, r)
		})
	}
}

func (s *Server) writeReadOnlyError(w http.ResponseWriter) {
	if s.config.ReadOnly {
		writeError(w, http.StatusForbidden, "Server is in read-only mode")
		return
	}
}

func (s *Server) getPath(r *http.Request) string {
	p := r.URL.Query().Get("p")
	if p == "" {
		p = "/"
	}
	return p
}

func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}