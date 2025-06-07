package main


import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"tritontube/internal/proto"
	"tritontube/internal/storage"
)

func main() {
	host := flag.String("host", "localhost", "Host address for the server")
	port := flag.Int("port", 8090, "Port number for the server")
	flag.Parse()

	// Validate arguments
	if *port <= 0 {
		panic("Error: Port number must be positive")
	}

	if flag.NArg() < 1 {
		fmt.Println("Usage: storage [OPTIONS] <baseDir>")
		fmt.Println("Error: Base directory argument is required")
		return
	}
	baseDir := flag.Arg(0)


	fmt.Println("Starting storage server...")
	fmt.Printf("Host: %s\n", *host)
	fmt.Printf("Port: %d\n", *port)
	fmt.Printf("Base Directory: %s\n", baseDir)

	addr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	grpcServer := grpc.NewServer()
	service := &storage.VideoStorageService{BaseDir: baseDir}
	proto.RegisterVideoStorageServiceServer(grpcServer, service)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	panic("Lab 8: not implemented")
}
