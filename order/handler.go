package main

import (
	"context"

	pb "github.com/two-phase-commit-demo/order/proto"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedOrderServiceServer
}

func NewGRPCHandler(server *grpc.Server) {
	handler := &grpcHandler{}
	pb.RegisterOrderServiceServer(server, handler)
}

func (h *grpcHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	return nil, nil
}