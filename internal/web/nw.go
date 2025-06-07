// Lab 8: Implement a network video content service (client using consistent hashing)

package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"tritontube/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NetworkVideoContentService struct {
	proto.UnimplementedVideoContentAdminServiceServer
	nodes      []string
	hashRing   []uint64
	hashToAddr map[uint64]string
	clients    map[string]proto.VideoStorageServiceClient
	files      map[string][]byte // Simulated in-memory store
}

var _ VideoContentService = (*NetworkVideoContentService)(nil)
var _ proto.VideoContentAdminServiceServer = (*NetworkVideoContentService)(nil)

func NewNetworkVideoContentService(option string) (*NetworkVideoContentService, error) {
	nodes := strings.Split(option, ",")
	if len(nodes) == 0 {
		return nil, errors.New("must provide at least one storage node")
	}

	s := &NetworkVideoContentService{
		nodes:      nodes,
		hashToAddr: make(map[uint64]string),
		clients:    make(map[string]proto.VideoStorageServiceClient),
		files:      make(map[string][]byte),
	}

	for _, addr := range nodes {
		h := hashStringToUint64(addr)
		s.hashRing = append(s.hashRing, h)
		s.hashToAddr[h] = addr

		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to dial %s: %v", addr, err)
		}
		s.clients[addr] = proto.NewVideoStorageServiceClient(conn)
	}

	s.sortRing()
	return s, nil
}

func (s *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := videoId + "/" + filename
	addr := s.findNode(key)
	client := s.clients[addr]
	resp, err := client.ReadFile(context.Background(), &proto.ReadFileRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *NetworkVideoContentService) Write(videoId, filename string, data []byte) error {
	key := videoId + "/" + filename
	addr := s.findNode(key)
	client := s.clients[addr]
	_, err := client.WriteFile(context.Background(), &proto.WriteFileRequest{
		VideoId:  videoId,
		Filename: filename,
		Data:     data,
	})
	if err == nil {
		s.files[key] = data // for migration only
	}
	return err
}

func (s *NetworkVideoContentService) ListNodes(ctx context.Context, req *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	return &proto.ListNodesResponse{Nodes: s.nodes}, nil
}

func (s *NetworkVideoContentService) AddNode(ctx context.Context, req *proto.AddNodeRequest) (*proto.AddNodeResponse, error) {
	addr := req.NodeAddress

	// Save old ring state
	oldRing := append([]uint64{}, s.hashRing...)
	oldHashToAddr := map[uint64]string{}
	for _, h := range oldRing {
		oldHashToAddr[h] = s.hashToAddr[h]
	}
	sort.Slice(oldRing, func(i, j int) bool {
		return oldRing[i] < oldRing[j]
	})

	// Add node
	for _, existing := range s.nodes {
		if existing == addr {
			return nil, errors.New("node already exists")
		}
	}
	s.nodes = append(s.nodes, addr)
	h := hashStringToUint64(addr)
	s.hashRing = append(s.hashRing, h)
	s.hashToAddr[h] = addr

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to new node: %v", err)
	}
	s.clients[addr] = proto.NewVideoStorageServiceClient(conn)
	s.sortRing()

	migrated := 0
	for key, data := range s.files {
		oldNode := findNodeInStaticRing(key, oldRing, oldHashToAddr)
		newNode := s.findNode(key)
		if newNode == addr && oldNode != newNode {
			parts := strings.SplitN(key, "/", 2)
			if len(parts) != 2 {
				continue
			}
			videoId, filename := parts[0], parts[1]
			_, err := s.clients[newNode].WriteFile(context.Background(), &proto.WriteFileRequest{
				VideoId:  videoId,
				Filename: filename,
				Data:     data,
			})
			if err == nil {
				s.clients[oldNode].DeleteFile(context.Background(), &proto.DeleteFileRequest{
					VideoId:  videoId,
					Filename: filename,
				})
				migrated++
			}
		}
	}

	return &proto.AddNodeResponse{MigratedFileCount: int32(migrated)}, nil
}

func (s *NetworkVideoContentService) RemoveNode(ctx context.Context, req *proto.RemoveNodeRequest) (*proto.RemoveNodeResponse, error) {
	addr := req.NodeAddress

	// Save reference to removed node client
	oldClient := s.clients[addr]

	oldRing := append([]uint64{}, s.hashRing...)
	oldHashToAddr := map[uint64]string{}
	for _, h := range oldRing {
		oldHashToAddr[h] = s.hashToAddr[h]
	}
	sort.Slice(oldRing, func(i, j int) bool {
		return oldRing[i] < oldRing[j]
	})

	// Remove node from internal structures
	newNodes := []string{}
	for _, n := range s.nodes {
		if n != addr {
			newNodes = append(newNodes, n)
		}
	}
	s.nodes = newNodes

	newHashRing := []uint64{}
	for _, h := range s.hashRing {
		if s.hashToAddr[h] != addr {
			newHashRing = append(newHashRing, h)
		}
	}
	s.hashRing = newHashRing
	delete(s.clients, addr)
	s.sortRing()

	migrated := 0
	for key, data := range s.files {
		oldNode := findNodeInStaticRing(key, oldRing, oldHashToAddr)
		if oldNode != addr {
			continue
		}
		newNode := s.findNode(key)
		if newNode == addr {
			continue
		}

		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		videoId, filename := parts[0], parts[1]

		_, err := s.clients[newNode].WriteFile(context.Background(), &proto.WriteFileRequest{
			VideoId:  videoId,
			Filename: filename,
			Data:     data,
		})
		if err == nil {
			oldClient.DeleteFile(context.Background(), &proto.DeleteFileRequest{
				VideoId:  videoId,
				Filename: filename,
			})
			migrated++
		}
	}

	return &proto.RemoveNodeResponse{MigratedFileCount: int32(migrated)}, nil
}

func (s *NetworkVideoContentService) findNode(key string) string {
	h := hashStringToUint64(key)
	for _, ringHash := range s.hashRing {
		if h <= ringHash {
			return s.hashToAddr[ringHash]
		}
	}
	return s.hashToAddr[s.hashRing[0]]
}

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}

func findNodeInStaticRing(key string, ring []uint64, hashToAddr map[uint64]string) string {
	h := hashStringToUint64(key)
	for _, ringHash := range ring {
		if h <= ringHash {
			return hashToAddr[ringHash]
		}
	}
	return hashToAddr[ring[0]]
}

func (s *NetworkVideoContentService) sortRing() {
	sort.Slice(s.hashRing, func(i, j int) bool {
		return s.hashRing[i] < s.hashRing[j]
	})
}
