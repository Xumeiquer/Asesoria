// Package storage stores uploaded documents content-addressed by SHA-256.
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	Root string
}

func New(root string) (*Storage, error) {
	for _, dir := range []string{root, filepath.Join(root, "documents"), filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create storage dir %s: %w", dir, err)
		}
	}
	return &Storage{Root: root}, nil
}

// Save streams r to disk and returns the relative path, hash and size.
func (s *Storage) Save(r io.Reader) (relPath, hash string, size int64, err error) {
	tmp, err := os.CreateTemp(filepath.Join(s.Root, "tmp"), "upload-*")
	if err != nil {
		return "", "", 0, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	h := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, 64<<20))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", "", 0, err
	}
	hash = hex.EncodeToString(h.Sum(nil))
	relPath = filepath.Join("documents", hash[:2], hash)
	dest := filepath.Join(s.Root, relPath)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", "", 0, err
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		return relPath, hash, size, nil
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return "", "", 0, err
	}
	if err := os.Chmod(dest, 0o600); err != nil {
		return "", "", 0, err
	}
	return relPath, hash, size, nil
}

// Open opens a stored file, rejecting paths outside the storage root.
func (s *Storage) Open(relPath string) (io.ReadSeekCloser, error) {
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return nil, fmt.Errorf("invalid storage path")
	}
	return os.Open(filepath.Join(s.Root, clean))
}

func (s *Storage) Remove(relPath string) error {
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return fmt.Errorf("invalid storage path")
	}
	err := os.Remove(filepath.Join(s.Root, clean))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
