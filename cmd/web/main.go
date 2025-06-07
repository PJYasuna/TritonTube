package main

import (
	"flag"
	"fmt"
	"net"
	"strings"
	"tritontube/internal/web"
	"tritontube/internal/proto"
	"google.golang.org/grpc"
)

// printUsage prints the usage information for the application
func printUsage() {
	fmt.Println("Usage: ./program [OPTIONS] METADATA_TYPE METADATA_OPTIONS CONTENT_TYPE CONTENT_OPTIONS")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  METADATA_TYPE         Metadata service type (sqlite, etcd)")
	fmt.Println("  METADATA_OPTIONS      Options for metadata service (e.g., db path)")
	fmt.Println("  CONTENT_TYPE          Content service type (fs, nw)")
	fmt.Println("  CONTENT_OPTIONS       Options for content service (e.g., base dir, network addresses)")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Example: ./program sqlite db.db fs /path/to/videos")
}

func main() {
	// Define flags
	port := flag.Int("port", 8080, "Port number for the web server")
	host := flag.String("host", "localhost", "Host address for the web server")

	// Set custom usage message
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if the correct number of positional arguments is provided
	if len(flag.Args()) != 4 {
		fmt.Println("Error: Incorrect number of arguments")
		printUsage()
		return
	}

	// Parse positional arguments
	metadataServiceType := flag.Arg(0)
	metadataServiceOptions := flag.Arg(1)
	contentServiceType := flag.Arg(2)
	contentServiceOptions := flag.Arg(3)

	// Validate port number (already an int from flag, check if positive)
	if *port <= 0 {
		fmt.Println("Error: Invalid port number:", *port)
		printUsage()
		return
	}

	// Construct metadata service
	var metadataService web.VideoMetadataService
	fmt.Println("Creating metadata service of type", metadataServiceType, "with options", metadataServiceOptions)
	switch metadataServiceType {
	case "sqlite":
		var err error
		metadataService, err = web.NewSQLiteVideoMetadataService(metadataServiceOptions)
		if err != nil {
			fmt.Println("Error creating SQLite metadata service:", err)
			return
		}
	default:
		fmt.Println("Unsupported metadata type:", metadataServiceType)
		return
	}

	// Construct content service
	var contentService web.VideoContentService
	var err error
	fmt.Println("Creating content service of type", contentServiceType, "with options", contentServiceOptions)

	switch contentServiceType {
	case "fs":
		contentService = web.NewFSVideoContentService(contentServiceOptions)
	case "nw":
		parts := strings.Split(contentServiceOptions, ",")
		if len(parts) < 2 {
			fmt.Println("Error: content options should be adminAddr,storageAddr1,...")
			return
		}
		adminAddr := parts[0]
		storageNodes := strings.Join(parts[1:], ",")

		contentService, err = web.NewNetworkVideoContentService(storageNodes)
		if err != nil {
			fmt.Println("Error creating network content service:", err)
			return
		}

		// Start gRPC admin server
		// go func() {
		// 	lis, err := net.Listen("tcp", adminAddr)
		// 	if err != nil {
		// 		fmt.Println("Failed to listen on adminAddr:", err)
		// 		return
		// 	}
		// 	grpcServer := grpc.NewServer()
		// 	proto.RegisterVideoContentAdminServiceServer(grpcServer, contentService.(proto.VideoContentAdminServiceServer))
		// 	fmt.Println("gRPC admin server started on", adminAddr)
		// 	if err := grpcServer.Serve(lis); err != nil {
		// 		fmt.Println("gRPC server error:", err)
		// 	}
		// }()
		ready := make(chan struct{})
		go func() {
			lis, err := net.Listen("tcp", adminAddr)
			if err != nil {
				fmt.Println("Failed to listen on adminAddr:", err)
				close(ready) // avoid deadlock
				return
			}
			grpcServer := grpc.NewServer()
			proto.RegisterVideoContentAdminServiceServer(grpcServer, contentService.(proto.VideoContentAdminServiceServer))
			fmt.Println("gRPC admin server started on", adminAddr)
		
			close(ready) // signal ready
			if err := grpcServer.Serve(lis); err != nil {
				fmt.Println("gRPC server error:", err)
			}
		}()
		<-ready
	default:
		fmt.Println("Unsupported content type:", contentServiceType)
		return
	}

	// Start the server
	server := web.NewServer(metadataService, contentService)
	listenAddr := fmt.Sprintf("%s:%d", *host , *port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer lis.Close()

	fmt.Println("Starting web server on", listenAddr)
	err = server.Start(lis)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
}
