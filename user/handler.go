package main

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/lib/pq"
	pb "github.com/two-phase-commit-demo/user/proto"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedUserServiceServer

	db *sql.DB
}

func NewGRPCHandler(server *grpc.Server) {
	db, err := sql.Open("postgres", "host=user-db port=5432 user=postgres password=sample_password dbname=user sslmode=disable")
	if err != nil {
		log.Fatalf("connect db error: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db error: %v", err)
	}

	query := `
		CREATE TABLE IF NOT EXISTS "users" (
			id VARCHAR(1024) PRIMARY KEY,
			balance INT
		);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("table created failed: %v\n", err)
	}

	id := "04937668-e73f-4035-a7d7-8f8db1a679e8"
	query = "INSERT INTO users (id, balance) VALUES ($1, $2)"
	_, err = db.Exec(query, id, 10000)
	if err != nil {
		log.Printf("insert user failed: %v\n", err)
	}

	handler := &grpcHandler{db: db}
	pb.RegisterUserServiceServer(server, handler)
}

func (h *grpcHandler) DeductWallet(ctx context.Context, req *pb.DeductWalletRequest) (*pb.DeductWalletResponse, error) {
	log.Println("user service: deduct wallet")

	query := `
		UPDATE users
		SET balance = balance - $1
		WHERE id = $2
	`
	_, err := h.db.Exec(query, req.Price, req.UserId)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
