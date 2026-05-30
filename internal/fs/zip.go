package fs

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZipDir streams a zip archive of the directory at virtualPath to w.
// Symlinks are skipped to avoid loops and targets outside the root.
func (fm *FileManager) ZipDir(virtualPath string, w io.Writer) error {
	absRoot, err := fm.pathHandler.SafePath(virtualPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	return filepath.WalkDir(absRoot, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == absRoot {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(absRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		fi, err := d.Info()
		if err != nil {
			return nil
		}

		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}

		if d.IsDir() {
			header.Name = rel + "/"
			header.Method = zip.Store
			_, err = zw.CreateHeader(header)
			return err
		}

		header.Name = rel
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(writer, f)
		return err
	})
}
