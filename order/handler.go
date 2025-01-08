package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	pb "github.com/Alvintan0712/two-phase-commit-demo/shared/api/proto"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/transaction"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/zkclient"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type grpcHandler struct {
	pb.UnimplementedOrderServiceServer

	db *sql.DB
}

type transactionHandler struct {
	serviceName string
	db          *sql.DB
	client      *zkclient.ZooKeeperClient
	watcher     transaction.TransactionWatcher
}

func NewHandler(server *grpc.Server, watcher transaction.TransactionWatcher, zkClient *zkclient.ZooKeeperClient) {
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

	registerTransactionHandlers(db, watcher, zkClient)
}

func registerTransactionHandlers(db *sql.DB, watcher transaction.TransactionWatcher, client *zkclient.ZooKeeperClient) {
	txHandler := &transactionHandler{
		serviceName: "order",
		db:          db,
		client:      client,
		watcher:     watcher,
	}

	watcher.RegisterHandler(transaction.OrderCreation, txHandler.prepareCreateOrder, txHandler.finalizeCreateOrder)

	watcher.Watch()
}

func (h *transactionHandler) prepareCreateOrder(txData transaction.TransactionData) error {
	log.Println("order service: 2pc create order")

	// Ensure the transaction is prepared
	path := h.watcher.GetBasePath() + "/" + string(transaction.OrderCreation) + "/" + txData.Id + "/" + h.serviceName
	for {
		data, ch, err := h.client.GetW(path)
		if err != nil {
			return fmt.Errorf("error in set %s watches: %v", path, err)
		}

		if string(data) == string(transaction.StatusPrepared) {
			break
		}

		if string(data) != string(transaction.StatusInit) {
			return nil
		}

		<-ch
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("error in begin transaction: %v\n", err)
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return fmt.Errorf("error in begin transaction: %v", err)
	}

	var data *pb.PlaceOrderRequest

	id := uuid.New().String()
	if err := json.Unmarshal(txData.Payload, &data); err != nil {
		log.Printf("error in unmarshal payload: %v\n", err)
		return fmt.Errorf("error in unmarshal payload: %v", err)
	}

	query := `INSERT INTO orders (id, user_id, price) VALUES ($1, $2, $3)`
	_, err = tx.Exec(query, id, data.UserId, data.Price)
	if err != nil {
		log.Printf("error in execute insert order: %v\n", err)
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return err
	}

	query = fmt.Sprintf("PREPARE TRANSACTION '%s'", txData.Id)
	_, err = tx.Exec(query)
	if err != nil {
		log.Printf("error in execute prepare statement: %v\n", err)
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return fmt.Errorf("error in execute prepare statement: %v", err)
	}

	log.Println("write znode value")
	err = h.client.Set(path, []byte(transaction.StatusReady))
	if err != nil {
		log.Printf("error in write in zookeeper: %v\n", err)
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return fmt.Errorf("error in write in zookeeper: %v", err)
	}

	log.Println("order service ready")

	return nil
}

func (h *transactionHandler) finalizeCreateOrder(txId string) error {
	log.Println("Finalize create order transaction")
	path := h.watcher.GetBasePath() + "/" + string(transaction.OrderCreation) + "/" + txId + "/" + h.serviceName
	for {
		data, ch, err := h.client.GetW(path) // watches transaction znode value
		if err != nil {
			log.Printf("error in set %s watches: %v\n", path, err)
			return fmt.Errorf("error in set %s watches: %v", path, err)
		}

		switch string(data) {
		case string(transaction.StatusCommit):
			log.Println("Commit create order transaction")
			query := fmt.Sprintf("COMMIT PREPARED '%s'", txId)
			_, err := h.db.Exec(query)
			if err != nil {
				return fmt.Errorf("error in commit prepared transaction %s: %v", txId, err)
			}
			err = h.client.Set(path, []byte(transaction.StatusCommitted))
			if err != nil {
				return fmt.Errorf("error in set znode value: %v", err)
			}
			return nil
		case string(transaction.StatusRollBack):
			log.Println("Rollback create order transaction")
			query := fmt.Sprintf("ROLLBACK PREPARED '%s'", txId)
			_, err := h.db.Exec(query)
			if err != nil {
				return fmt.Errorf("error in rollback prepared transaction %s: %v", txId, err)
			}
			err = h.client.Set(path, []byte(transaction.StatusRolledBack))
			if err != nil {
				return fmt.Errorf("error in set znode value: %v", err)
			}
			return nil
		default:
			log.Printf("transaction data: %v\n", string(data))
		}

		<-ch
	}
}

func (h *transactionHandler) rollback(tx *sql.Tx, txId string, txType transaction.TransactionType) error {
	path := h.watcher.GetBasePath() + "/" + string(txType) + "/" + txId + "/" + h.serviceName
	tx.Rollback()
	err := h.client.Set(path, []byte(transaction.StatusAbort))
	if err != nil {
		return err
	}

	return nil
}

func (h *grpcHandler) GetOrders(ctx context.Context, req *emptypb.Empty) (*pb.GetOrdersResponse, error) {
	log.Println("order service: get orders")

	rows, err := h.db.Query("SELECT id, price FROM orders")
	if err != nil {
		log.Printf("error in query orders: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	var orders []*pb.Order
	for rows.Next() {
		var order pb.Order
		if err := rows.Scan(&order.Id, &order.Price); err != nil {
			log.Printf("error in scan order: %v\n", err)
			return nil, err
		}

		orders = append(orders, &order)
	}

	return &pb.GetOrdersResponse{Orders: orders}, nil
}
