package main

import (
	"context"

	pb "github.com/two-phase-commit-demo/user/proto"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedUserServiceServer
}

func NewGRPCHandler(server *grpc.Server) {
	handler := &grpcHandler{}
	pb.RegisterUserServiceServer(server, handler)
}

func (h *grpcHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	return nil, nil
}
