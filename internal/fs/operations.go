package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	MTime    time.Time `json:"mtime"`
	MIME     string    `json:"mime,omitempty"`
	Ext      string    `json:"ext,omitempty"`
	Mode     string    `json:"mode,omitempty"`
	SHA256   string    `json:"sha256,omitempty"`
	ETag     string    `json:"etag,omitempty"`
}

type FileManager struct {
	pathHandler *PathHandler
}

func NewFileManager(root string) (*FileManager, error) {
	ph, err := NewPathHandler(root)
	if err != nil {
		return nil, err
	}
	return &FileManager{pathHandler: ph}, nil
}

func (fm *FileManager) Root() string {
	return fm.pathHandler.Root()
}

func (fm *FileManager) List(virtualPath string) ([]FileInfo, error) {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryPath := filepath.Join(absPath, entry.Name())
		virtualEntryPath, err := fm.pathHandler.VirtualPath(entryPath)
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:  entry.Name(),
			Path:  virtualEntryPath,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
			MTime: info.ModTime(),
		}

		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			fileInfo.Ext = ext
			fileInfo.MIME = mime.TypeByExtension(ext)
			
			if fileInfo.MIME == "" {
				fileInfo.MIME = fm.detectMIME(entryPath)
			}
		}

		files = append(files, fileInfo)
	}

	return fm.sortFiles(files), nil
}

func (fm *FileManager) Find(virtualPath, query string, limit int) ([]FileInfo, bool, error) {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return nil, false, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("not a directory")
	}

	q := strings.ToLower(query)
	var results []FileInfo
	truncated := false

	err = filepath.WalkDir(absPath, func(entryPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entryPath == absPath {
			return nil
		}
		if !strings.Contains(strings.ToLower(d.Name()), q) {
			return nil
		}

		entryInfo, err := d.Info()
		if err != nil {
			return nil
		}

		virtualEntryPath, err := fm.pathHandler.VirtualPath(entryPath)
		if err != nil {
			return nil
		}

		fi := FileInfo{
			Name:  d.Name(),
			Path:  virtualEntryPath,
			IsDir: d.IsDir(),
			Size:  entryInfo.Size(),
			MTime: entryInfo.ModTime(),
		}
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			fi.Ext = ext
			fi.MIME = mime.TypeByExtension(ext)
		}
		results = append(results, fi)

		if limit > 0 && len(results) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return results, truncated, nil
}

func (fm *FileManager) Stat(virtualPath string) (*FileInfo, error) {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	fileInfo := &FileInfo{
		Name:  info.Name(),
		Path:  virtualPath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
		MTime: info.ModTime(),
		Mode:  info.Mode().String(),
	}

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(info.Name()))
		fileInfo.Ext = ext
		fileInfo.MIME = mime.TypeByExtension(ext)
		
		if fileInfo.MIME == "" {
			fileInfo.MIME = fm.detectMIME(absPath)
		}

		hash, err := fm.calculateSHA256(absPath)
		if err == nil {
			fileInfo.SHA256 = hash
			fileInfo.ETag = `"` + hash[:16] + `"`
		}
	}

	return fileInfo, nil
}

func (fm *FileManager) OpenFile(virtualPath string) (*os.File, *FileInfo, error) {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, nil, err
	}

	// Use StatFast to avoid expensive SHA256 calculation for streaming
	info, err := fm.StatFast(virtualPath)
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	return file, info, nil
}

// StatFast returns file info without calculating SHA256 hash.
// Uses size and mtime for ETag, suitable for streaming large files.
func (fm *FileManager) StatFast(virtualPath string) (*FileInfo, error) {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	fileInfo := &FileInfo{
		Name:  info.Name(),
		Path:  virtualPath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
		MTime: info.ModTime(),
		Mode:  info.Mode().String(),
	}

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(info.Name()))
		fileInfo.Ext = ext
		fileInfo.MIME = mime.TypeByExtension(ext)

		if fileInfo.MIME == "" {
			fileInfo.MIME = fm.detectMIME(absPath)
		}

		// Fast ETag based on size and mtime (no SHA256)
		fileInfo.ETag = fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano())
	}

	return fileInfo, nil
}

func (fm *FileManager) CreateDir(parentVirtualPath, name string) error {
	parentAbsPath, err := fm.pathHandler.SafePath(parentVirtualPath)
	if err != nil {
		return err
	}

	newDirPath := filepath.Join(parentAbsPath, name)
	return os.Mkdir(newDirPath, 0755)
}

func (fm *FileManager) Delete(virtualPath string, recursive bool) error {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	if info.IsDir() && !recursive {
		return fmt.Errorf("cannot delete directory without recursive flag")
	}

	if recursive {
		return os.RemoveAll(absPath)
	}
	return os.Remove(absPath)
}

func (fm *FileManager) Rename(fromVirtualPath, toVirtualPath string) error {
	fromAbsPath, err := fm.pathHandler.SafePath(fromVirtualPath)
	if err != nil {
		return err
	}

	toAbsPath, err := fm.pathHandler.SafePath(toVirtualPath)
	if err != nil {
		return err
	}

	return os.Rename(fromAbsPath, toAbsPath)
}

func (fm *FileManager) WriteFile(virtualPath string, src io.Reader) error {
	absPath, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return err
	}

	tempPath := absPath + ".websz.tmp"
	
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, src)
	if err != nil {
		os.Remove(tempPath)
		return err
	}

	return os.Rename(tempPath, absPath)
}

func (fm *FileManager) detectMIME(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return ""
	}

	return http.DetectContentType(buffer[:n])
}

func (fm *FileManager) calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fm *FileManager) sortFiles(files []FileInfo) []FileInfo {
	dirs := []FileInfo{}
	regularFiles := []FileInfo{}

	for _, file := range files {
		if file.IsDir {
			dirs = append(dirs, file)
		} else {
			regularFiles = append(regularFiles, file)
		}
	}

	result := make([]FileInfo, 0, len(files))
	result = append(result, dirs...)
	result = append(result, regularFiles...)

	return result
}