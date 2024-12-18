package main

import (
	"fmt"
	"log"
	"net"
	"syscall"

	"google.golang.org/grpc"
)

func main() {
	host, ok := syscall.Getenv("HOST")
	if !ok {
		host = "127.0.0.1"
	}

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:8081", host))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listen.Close()

	server := grpc.NewServer()
	NewGRPCHandler(server)

	log.Printf("Order service started at %s:8081\n", host)

	if err := server.Serve(listen); err != nil {
		log.Fatal(err)
	}
}
