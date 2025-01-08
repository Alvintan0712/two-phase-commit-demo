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
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

type grpcHandler struct {
	pb.UnimplementedUserServiceServer

	db *sql.DB
}

type transactionHandler struct {
	serviceName string
	db          *sql.DB
	client      *zkclient.ZooKeeperClient
	watcher     transaction.TransactionWatcher
}

func NewHandler(server *grpc.Server, watcher transaction.TransactionWatcher, zkClient *zkclient.ZooKeeperClient) {
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

	registerTransactionHandlers(db, watcher, zkClient)
}

func registerTransactionHandlers(db *sql.DB, watcher transaction.TransactionWatcher, client *zkclient.ZooKeeperClient) {
	txHandler := &transactionHandler{
		serviceName: "user",
		db:          db,
		client:      client,
		watcher:     watcher,
	}

	watcher.RegisterHandler(transaction.OrderCreation, txHandler.prepareDeductBalance, txHandler.finalizeDeductBalance)

	watcher.Watch()
}

func (h *transactionHandler) prepareDeductBalance(txData transaction.TransactionData) error {
	log.Println("user service: 2pc deduct wallet")

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
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return fmt.Errorf("error in start transaction: %v", err)
	}

	var data *pb.PlaceOrderRequest
	var user User

	if err := json.Unmarshal(txData.Payload, &data); err != nil {
		return fmt.Errorf("error in unmarshal payload: %v", err)
	}

	query := "SELECT id, balance FROM users WHERE id = $1 FOR UPDATE"
	row := tx.QueryRow(query, data.UserId)
	if err := row.Scan(&user.Id, &user.Balance); err != nil {
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		if err == sql.ErrNoRows {
			return fmt.Errorf("user id %s not found", data.UserId)
		}
		return err
	}

	if user.Balance < int(data.Price) {
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return fmt.Errorf("error insufficient wallet balance")
	}

	query = `
		UPDATE users
		SET balance = balance - $1
		WHERE id = $2
	`
	if _, err := tx.Exec(query, data.Price, data.UserId); err != nil {
		h.rollback(tx, txData.Id, transaction.OrderCreation)
		return err
	}

	query = fmt.Sprintf("PREPARE TRANSACTION '%s'", txData.Id)
	_, err = tx.Exec(query)
	if err != nil {
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

	log.Println("user service ready")

	return nil
}

func (h *transactionHandler) finalizeDeductBalance(txId string) error {
	path := h.watcher.GetBasePath() + "/" + string(transaction.OrderCreation) + "/" + txId + "/" + h.serviceName
	for {
		data, ch, err := h.client.GetW(path) // watches transaction znode value
		if err != nil {
			return fmt.Errorf("error in set %s watches: %v", path, err)
		}

		switch string(data) {
		case string(transaction.StatusCommit):
			log.Println("Commit deduct balance transaction")
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
			log.Println("Rollback deduct balance transaction")
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

func (h *grpcHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	log.Println("user service: get user")

	var resp pb.GetUserResponse

	query := `
		SELECT id, balance FROM users
		WHERE id = $1
	`
	row := h.db.QueryRow(query, req.UserId)
	if err := row.Scan(&resp.Id, &resp.Balance); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user id %s not found", req.UserId)
		}
		return nil, fmt.Errorf("user id %s: %v", req.UserId, err)
	}

	return &resp, nil
}
