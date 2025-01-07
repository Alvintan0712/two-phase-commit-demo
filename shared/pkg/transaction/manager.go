package transaction

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/zkclient"
	"github.com/go-zookeeper/zk"
	"github.com/google/uuid"
)

type transactionManager struct {
	client   *zkclient.ZooKeeperClient
	basePath string
	lockPath string
}

func NewTransactionManager(client *zkclient.ZooKeeperClient) (*transactionManager, error) {
	tm := &transactionManager{
		client:   client,
		basePath: "/transactions",
		lockPath: "/transactions/locks",
	}

	if err := tm.init(); err != nil {
		return nil, err
	}

	return tm, nil
}

// our isolation level is serialization
func (tm *transactionManager) Begin(txType TransactionType, payload []byte, participants []string, resources []ResourceType) (string, error) {
	txId := uuid.New().String()
	txPath := tm.basePath + "/" + string(txType) + "/" + txId
	txData := TransactionData{
		Id:        txId,
		Type:      txType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	data, err := json.Marshal(txData)
	if err != nil {
		return "", fmt.Errorf("error in marshal transaction data: %v", err)
	}

	if err := tm.client.Create(txPath, data); err != nil {
		return "", fmt.Errorf("error in create znode: %v", err)
	}

	for _, participant := range participants {
		path := txPath + "/" + participant
		if err := tm.client.Create(path, []byte(StatusInit)); err != nil {
			return "", fmt.Errorf("error in create participant znode: %v", err)
		}
	}

	return txId, nil
}

func (tm *transactionManager) Prepare(txId string) error {
	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId
		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return fmt.Errorf("error in check path: %v", err)
		}

		if exists {
			children, err := tm.client.Children(txPath)
			if err != nil {
				return fmt.Errorf("error in list children: %v", err)
			}

			for _, child := range children {
				path := txPath + "/" + child
				tm.client.Set(path, []byte(StatusPrepared))
			}
			return nil
		}
	}

	return fmt.Errorf("transaction id not found")
}

func (tm *transactionManager) GetVotesResult(txId string) (bool, error) {
	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId
		log.Printf("check %s\n", txPath)
		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return false, fmt.Errorf("error in check path: %v", err)
		}

		if exists {
			log.Printf("get %s votes results\n", txId)
			children, err := tm.client.Children(txPath)
			if err != nil {
				return false, fmt.Errorf("error in list children: %v", err)
			}

			var wg sync.WaitGroup
			isCommit := true
			for _, child := range children {
				path := txPath + "/" + child
				wg.Add(1)
				go func(znode string) {
					defer wg.Done()

					log.Printf("get %s votes results\n", znode)
					for {
						data, ch, err := tm.client.GetW(path)
						if err != nil {
							log.Printf("error in set watches %s: %v", path, err)
							continue
						}

						log.Printf("%s votes results: %v\n", znode, string(data))

						if string(data) != string(StatusPrepared) {
							if string(data) == string(StatusAbort) {
								isCommit = false
							}
							return
						}

						<-ch
					}
				}(path)
			}

			wg.Wait()

			return isCommit, nil
		}
	}

	return false, fmt.Errorf("transaction id not found")
}

func (tm *transactionManager) Finalize(txId string, isCommit bool) error {
	log.Println("finalize transaction " + txId)
	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId
		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return fmt.Errorf("error in check transaction path: %v", err)
		}

		value := StatusRollBack
		if isCommit {
			value = StatusCommit
		}

		if exists {
			children, err := tm.client.Children(txPath)
			if err != nil {
				return fmt.Errorf("error in list %s children: %v", txPath, err)
			}

			for _, child := range children {
				path := txPath + "/" + child
				tm.client.Set(path, []byte(value))
			}
			return nil
		}
	}

	return fmt.Errorf("transaction id not found")
}

func (tm *transactionManager) init() error {
	log.Println("init transaction znodes")
	if err := tm.client.Create(tm.basePath, []byte{}); err != nil {
		return err
	}

	for _, txType := range TransactionTypes {
		path := tm.basePath + "/" + string(txType)
		if err := tm.client.Create(path, []byte{}); err != nil {
			return err
		}
	}

	if err := tm.client.Create(tm.lockPath, []byte{}); err != nil {
		return err
	}

	log.Println("transaction znodes initialized")
	return nil
}

func (tm *transactionManager) acquireExclusiveLock(resources []ResourceType) error {
	start := time.Now()
	timeout := time.Duration(5 * time.Second)
	for i, resource := range resources {
		path := tm.lockPath + "/" + string(resource)
		for {
			err := tm.client.CreateEmphemeral(path, []byte{})
			if err == nil {
				break
			}
			if err == zk.ErrNodeExists {
				if time.Since(start) > timeout {
					log.Printf("Timeout while acquiring lock for %s\n", resource)
					return fmt.Errorf("timeout while acquiring lock for %s", resource)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			for j := 0; j < i; j++ {
				path := tm.lockPath + "/" + string(resources[j])
				if err := tm.releaseLock(path); err != nil {
					log.Printf("Error in releasing lock for %s: %v", resources[j], err)
				}
			}

			return fmt.Errorf("error in acquiring lock for %s: %v", resource, err)
		}
	}

	return nil
}

func (tm *transactionManager) releaseExclusiveLock(resources []ResourceType) error {
	for _, resource := range resources {
		path := tm.lockPath + "/" + string(resource)
		if err := tm.releaseLock(path); err != nil {
			return fmt.Errorf("error in releasing lock for %s: %v", resource, err)
		}
	}

	return nil
}

func (tm *transactionManager) releaseLock(nodePath string) error {
	return tm.client.Delete(nodePath)
}
