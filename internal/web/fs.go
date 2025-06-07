// Lab 7: Implement a local filesystem video content service

package web

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)


// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct{
	baseDir string
}

// Uncomment the following line to ensure FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)

func NewFSVideoContentService(baseDir string) *FSVideoContentService {
	return &FSVideoContentService{baseDir: baseDir}
}

func (s *FSVideoContentService) Write(videoId string, filename string, data []byte) error {
	dir := filepath.Join(s.baseDir, videoId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fullPath := filepath.Join(dir, filename)
	if err := ioutil.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func (s *FSVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	fullPath := filepath.Join(s.baseDir, videoId, filename)
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}