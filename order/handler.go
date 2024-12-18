package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	pb "github.com/two-phase-commit-demo/order/proto"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedOrderServiceServer

	db *sql.DB
}

func NewGRPCHandler(server *grpc.Server) {
	db, err := sql.Open("postgres", "host=order-db port=5432 user=postgres password=sample_password dbname=order sslmode=disable")
	if err != nil {
		log.Fatalf("connect db error: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db error: %v", err)
	}

	query := `
		CREATE TABLE IF NOT EXISTS "orders" (
			id VARCHAR(1024) PRIMARY KEY,
			user_id VARCHAR(1024),
			price INT
		);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("table created failed: %v\n", err)
	}

	handler := &grpcHandler{db: db}
	pb.RegisterOrderServiceServer(server, handler)
}

func (h *grpcHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	log.Println("order service: create order")

	id := uuid.New().String()
	query := `INSERT INTO orders (id, user_id, price) VALUES ($1, $2, $3)`
	_, err := h.db.Exec(query, id, req.UserId, req.Price)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
