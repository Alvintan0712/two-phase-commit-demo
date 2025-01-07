package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/grpc"

	pb "github.com/Alvintan0712/two-phase-commit-demo/shared/api/proto"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/transaction"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/zkclient"
)

type grpcHandler struct {
	pb.UnimplementedCoordinatorServiceServer

	zkClient *zkclient.ZooKeeperClient
	tw       transaction.TransactionWatcher
	tm       transaction.TransactionManager
}

func NewHandler(
	server *grpc.Server,
	zkClient *zkclient.ZooKeeperClient,
	tw transaction.TransactionWatcher,
	tm transaction.TransactionManager) {

	handler := &grpcHandler{
		tw:       tw,
		tm:       tm,
		zkClient: zkClient,
	}
	pb.RegisterCoordinatorServiceServer(server, handler)
}

func (h *grpcHandler) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	log.Println("coordinator: place order request")
	participants := []string{"order", "user"}
	resources := []transaction.ResourceType{transaction.OrderResource, transaction.UserResource}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error in marshal place order request: %v", err)
	}

	txId, err := h.tm.Begin(transaction.OrderCreation, data, participants, resources)
	if err != nil {
		return nil, err
	}

	err = h.tm.Prepare(txId)
	if err != nil {
		return nil, err
	}

	isCommit, err := h.tm.GetVotesResult(txId)
	if err != nil {
		return nil, err
	}

	err = h.tm.Finalize(txId, isCommit)
	if err != nil {
		return nil, err
	}

	if !isCommit {
		return &pb.PlaceOrderResponse{Message: "order place failed", Success: false}, nil
	}

	return &pb.PlaceOrderResponse{Message: "order placed successfully", Success: true}, nil
}
