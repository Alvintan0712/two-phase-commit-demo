package main

import (
	"fmt"
	"log"
	"net"
	"syscall"

	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/transaction"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/zkclient"
	"google.golang.org/grpc"
)

func main() {
	host, ok := syscall.Getenv("HOST")
	if !ok {
		host = "127.0.0.1"
	}

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:8082", host))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listen.Close()

	zkServer, ok := syscall.Getenv("ZK_SERVER")
	if !ok {
		zkServer = "127.0.0.1:2181"
	}

	zkClient, err := zkclient.NewZooKeeperClient([]string{zkServer})
	if err != nil {
		log.Fatal(err)
	}
	defer zkClient.Close()

	tm, err := transaction.NewTransactionManager(zkClient)
	if err != nil {
		log.Fatal(err)
	}

	tw, err := transaction.NewTransactionWatcher(zkClient)
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	NewHandler(server, zkClient, tw, tm)

	log.Printf("Coordinator service started at %s:8082\n", host)

	if err := server.Serve(listen); err != nil {
		log.Fatal(err)
	}
}
