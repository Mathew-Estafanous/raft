package main

import (
	"google.golang.org/grpc"
	"log"
	"net"

)

func main() {
	lis, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()

	err = server.Serve(lis)
	if err != nil {
		log.Fatalf("Failed to serve grpc: %v", err)
	}
}
