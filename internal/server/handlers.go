package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.handleMethodNotAllowed(w, "GET")
		return
	}

	p := s.getPath(r)
	files, err := s.fm.List(p)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "Directory not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": files,
	})
}

func (s *Server) handleStat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.handleMethodNotAllowed(w, "GET")
		return
	}

	p := s.getPath(r)
	info, err := s.fm.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "File not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.handleMethodNotAllowed(w, "GET")
		return
	}

	p := s.getPath(r)
	file, info, err := s.fm.OpenFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "File not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	if info.IsDir {
		writeError(w, http.StatusBadRequest, "Cannot download directory")
		return
	}

	w.Header().Set("Content-Type", info.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	
	if info.ETag != "" {
		w.Header().Set("ETag", info.ETag)
	}

	http.ServeContent(w, r, info.Name, info.MTime, file)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.handleMethodNotAllowed(w, "GET")
		return
	}

	p := s.getPath(r)
	file, info, err := s.fm.OpenFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "File not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	if info.IsDir {
		writeError(w, http.StatusBadRequest, "Cannot open directory")
		return
	}

	w.Header().Set("Content-Type", info.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, info.Name))
	
	if info.ETag != "" {
		w.Header().Set("ETag", info.ETag)
	}

	http.ServeContent(w, r, info.Name, info.MTime, file)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.handleMethodNotAllowed(w, "POST")
		return
	}

	if s.config.ReadOnly {
		s.writeReadOnlyError(w)
		return
	}

	p := s.getPath(r)
	conflictStrategy := r.URL.Query().Get("conflict")
	if conflictStrategy == "" {
		conflictStrategy = "autorename"
	}

	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "No files provided")
		return
	}

	uploadedFiles := []string{}
	
	for _, fileHeader := range files {
		filename := s.resolveConflict(p, fileHeader.Filename, conflictStrategy)
		if filename == "" {
			writeError(w, http.StatusConflict, fmt.Sprintf("File %s already exists", fileHeader.Filename))
			return
		}

		targetPath := p
		if targetPath == "/" {
			targetPath = "/" + filename
		} else {
			targetPath = targetPath + "/" + filename
		}

		file, err := fileHeader.Open()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to open uploaded file")
			return
		}

		err = s.fm.WriteFile(targetPath, file)
		file.Close()
		
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file: %v", err))
			return
		}

		uploadedFiles = append(uploadedFiles, targetPath)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uploaded": uploadedFiles,
	})
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		s.handleMethodNotAllowed(w, "PUT")
		return
	}

	if s.config.ReadOnly {
		s.writeReadOnlyError(w)
		return
	}

	p := s.getPath(r)
	if p == "/" {
		writeError(w, http.StatusBadRequest, "Cannot PUT to root directory")
		return
	}

	err := s.fm.WriteFile(p, r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"path": p,
	})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.handleMethodNotAllowed(w, "POST")
		return
	}

	if s.config.ReadOnly {
		s.writeReadOnlyError(w)
		return
	}

	var req struct {
		P    string `json:"p"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Directory name required")
		return
	}

	err := s.fm.CreateDir(req.P, req.Name)
	if err != nil {
		if os.IsExist(err) {
			writeError(w, http.StatusConflict, "Directory already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	newPath := req.P
	if newPath == "/" {
		newPath = "/" + req.Name
	} else {
		newPath = newPath + "/" + req.Name
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"path": newPath,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.handleMethodNotAllowed(w, "POST")
		return
	}

	if s.config.ReadOnly {
		s.writeReadOnlyError(w)
		return
	}

	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err := s.fm.Rename(req.From, req.To)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "Source file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"from": req.From,
		"to":   req.To,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.handleMethodNotAllowed(w, "POST")
		return
	}

	if s.config.ReadOnly {
		s.writeReadOnlyError(w)
		return
	}

	var req struct {
		P         string `json:"p"`
		Recursive bool   `json:"recursive"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	err := s.fm.Delete(req.P, req.Recursive)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "File not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"deleted": req.P,
	})
}

func (s *Server) resolveConflict(parentPath, filename, strategy string) string {
	switch strategy {
	case "overwrite":
		return filename
	case "reject":
		testPath := parentPath
		if testPath == "/" {
			testPath = "/" + filename
		} else {
			testPath = testPath + "/" + filename
		}
		
		_, err := s.fm.Stat(testPath)
		if err == nil {
			return ""
		}
		return filename
	case "autorename":
		return s.autoRename(parentPath, filename)
	default:
		return filename
	}
}

func (s *Server) autoRename(parentPath, filename string) string {
	testPath := parentPath
	if testPath == "/" {
		testPath = "/" + filename
	} else {
		testPath = testPath + "/" + filename
	}
	
	_, err := s.fm.Stat(testPath)
	if err != nil {
		return filename
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for i := 1; i < 1000; i++ {
		newName := fmt.Sprintf("%s(%d)%s", base, i, ext)
		newPath := parentPath
		if newPath == "/" {
			newPath = "/" + newName
		} else {
			newPath = newPath + "/" + newName
		}
		
		_, err := s.fm.Stat(newPath)
		if err != nil {
			return newName
		}
	}

	return filename
}