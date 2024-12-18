package main

import (
	"context"
	"log"

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

func (h *grpcHandler) DeductWallet(ctx context.Context, req *pb.DeductWalletRequest) (*pb.DeductWalletResponse, error) {
	log.Println("user service: deduct wallet")
	return nil, nil
}
