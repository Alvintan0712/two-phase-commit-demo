package main

import (
	"context"
	"log"

	"google.golang.org/grpc"

	pb "github.com/two-phase-commit-demo/coordinator/proto"
)

type grpcHandler struct {
	pb.UnimplementedCoordinatorServiceServer
}

func NewGRPCHandler(server *grpc.Server) {
	handler := &grpcHandler{}
	pb.RegisterCoordinatorServiceServer(server, handler)
}

func (h *grpcHandler) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	log.Println("coordinator: place order request")
	return nil, nil
}
