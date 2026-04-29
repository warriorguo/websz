package fs

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type PathHandler struct {
	root string
}

func NewPathHandler(root string) (*PathHandler, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}
	return &PathHandler{root: absRoot}, nil
}

func (p *PathHandler) Root() string {
	return p.root
}

func (p *PathHandler) SafePath(virtualPath string) (string, error) {
	if virtualPath == "" {
		return p.root, nil
	}

	normalized := strings.ReplaceAll(virtualPath, "\\", "/")

	cleaned := path.Clean("/" + normalized)

	if strings.Contains(cleaned, "..") || strings.Contains(virtualPath, "..") {
		return "", fmt.Errorf("path traversal attempt: %s", virtualPath)
	}

	abs := filepath.Join(p.root, cleaned[1:])

	absClean := filepath.Clean(abs)

	rel, err := filepath.Rel(p.root, absClean)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("path traversal attempt: %s", virtualPath)
	}

	return absClean, nil
}

func (p *PathHandler) VirtualPath(absPath string) (string, error) {
	rel, err := filepath.Rel(p.root, absPath)
	if err != nil {
		return "", err
	}
	
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside root")
	}

	if rel == "." {
		return "/", nil
	}

	return "/" + strings.ReplaceAll(rel, "\\", "/"), nil
}