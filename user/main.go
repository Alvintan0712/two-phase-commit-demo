package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
)

func main() {
	listen, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listen.Close()

	server := grpc.NewServer()
	NewGRPCHandler(server)

	log.Println("User service started at 127.0.0.1:8080")

	if err := server.Serve(listen); err != nil {
		log.Fatal(err)
	}
}
