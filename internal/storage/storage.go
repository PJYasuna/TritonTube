// Lab 8: Implement a network video content service (server)

package storage

import (
	"context"
	"os"
	"path/filepath"

	"tritontube/internal/proto"
)

type VideoStorageService struct {
	proto.UnimplementedVideoStorageServiceServer
	BaseDir string
}

func (s *VideoStorageService) WriteFile(ctx context.Context, req *proto.WriteFileRequest) (*proto.WriteFileResponse, error) {
	path := filepath.Join(s.BaseDir, req.VideoId, req.Filename)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return &proto.WriteFileResponse{Success: false}, err
	}
	err = os.WriteFile(path, req.Data, 0644)
	if err != nil {
		return &proto.WriteFileResponse{Success: false}, err
	}
	return &proto.WriteFileResponse{Success: true}, nil
}

func (s *VideoStorageService) ReadFile(ctx context.Context, req *proto.ReadFileRequest) (*proto.ReadFileResponse, error) {
	path := filepath.Join(s.BaseDir, req.VideoId, req.Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &proto.ReadFileResponse{Data: data}, nil
}

func (s *VideoStorageService) DeleteFile(ctx context.Context, req *proto.DeleteFileRequest) (*proto.DeleteFileResponse, error) {
	path := filepath.Join(s.BaseDir, req.VideoId, req.Filename)
	err := os.Remove(path)
	if err != nil {
		return &proto.DeleteFileResponse{Success: false}, err
	}
	return &proto.DeleteFileResponse{Success: true}, nil
}
