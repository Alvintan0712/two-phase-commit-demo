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

	go tm.clean()

	return tm, nil
}

// our isolation level is serialization
func (tm *transactionManager) Begin(txType TransactionType, payload []byte, participants []string, resources []ResourceType) (string, error) {
	log.Printf("begin transaction %s\n", txType)

	txId := uuid.New().String()
	txPath := tm.basePath + "/" + string(txType) + "/" + txId
	txData := TransactionData{
		Id:           txId,
		Type:         txType,
		Timestamp:    time.Now(),
		Payload:      payload,
		Status:       StatusInit,
		Participants: participants,
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
	log.Printf("prepare transaction %s\n", txId)

	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId
		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return fmt.Errorf("error in check path: %v", err)
		}

		if exists {
			data, err := tm.client.Get(txPath)
			if err != nil {
				return fmt.Errorf("error in get znode %s: %v", txPath, err)
			}

			log.Printf("set %s status to prepared\n", txId)
			var txData TransactionData
			if err := json.Unmarshal(data, &txData); err != nil {
				return fmt.Errorf("error in unmarshal transaction %s data: %v", txId, err)
			}
			txData.Status = StatusPrepared
			data, err = json.Marshal(txData)
			if err != nil {
				return fmt.Errorf("error in marshal transaction %s data: %v", txId, err)
			}
			if err := tm.client.Set(txPath, data); err != nil {
				return fmt.Errorf("error in set znode %s value: %v", txPath, err)
			}

			children, err := tm.client.Children(txPath)
			if err != nil {
				return fmt.Errorf("error in list children: %v", err)
			}

			log.Printf("set %s participants status to prepared\n", txId)
			for _, child := range children {
				path := txPath + "/" + child
				tm.client.Set(path, []byte(StatusPrepared))
			}

			log.Printf("transaction %s prepared\n", txId)
			return nil
		}
	}

	return fmt.Errorf("transaction id not found")
}

func (tm *transactionManager) GetVotesResult(txId string) (bool, error) {
	log.Printf("get %s votes results\n", txId)

	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId

		log.Printf("check %s\n", txPath)
		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return false, fmt.Errorf("error in check path: %v", err)
		}

		if exists {
			log.Printf("get %s votes results\n", txId)
			data, err := tm.client.Get(txPath)
			if err != nil {
				return false, fmt.Errorf("error in get znode %s: %v", txPath, err)
			}

			var txData TransactionData
			if err := json.Unmarshal(data, &txData); err != nil {
				return false, fmt.Errorf("error in unmarshal transaction %s data: %v", txId, err)
			}

			var wg sync.WaitGroup
			isCommit := true
			for _, participant := range txData.Participants {
				path := txPath + "/" + participant
				wg.Add(1)
				go func(znode string) {
					defer wg.Done()

					log.Printf("get %s votes results\n", znode)
					for {
						data, ch, err := tm.client.GetW(path)
						if err != nil {
							if err == zk.ErrNoNode {
								log.Printf("znode %s not found\n", path)
							} else {
								log.Printf("error in set watches %s: %v", path, err)
							}
							isCommit = false
							return
						}

						log.Printf("%s votes results: %v\n", znode, string(data))

						if string(data) == string(StatusReady) {
							return
						} else if string(data) == string(StatusAbort) {
							isCommit = false
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

	value := StatusRollBack
	if isCommit {
		value = StatusCommit
	}

	for _, txType := range TransactionTypes {
		txPath := tm.basePath + "/" + string(txType) + "/" + txId

		exists, err := tm.client.Exists(txPath)
		if err != nil {
			return fmt.Errorf("error in check transaction path %s: %v", txPath, err)
		}

		if exists {
			data, err := tm.client.Get(txPath)
			if err != nil {
				return fmt.Errorf("error in get znode %s: %v", txPath, err)
			}

			var txData TransactionData
			if err := json.Unmarshal(data, &txData); err != nil {
				return fmt.Errorf("error in unmarshal transaction %s data: %v", txId, err)
			}
			txData.Status = value

			data, err = json.Marshal(txData)
			if err != nil {
				return fmt.Errorf("error in marshal transaction %s data: %v", txId, err)
			}

			log.Printf("write %s status to %s\n", txId, value)
			if err := tm.client.Set(txPath, data); err != nil {
				return fmt.Errorf("error in set znode %s value: %v", txPath, err)
			}

			for _, participant := range txData.Participants {
				path := txPath + "/" + participant
				data, err := tm.client.Get(path)
				if err != nil {
					return fmt.Errorf("error in get znode %s: %v", path, err)
				}

				if string(data) == string(StatusReady) {
					log.Printf("write %s/%s status to %s\n", txId, participant, value)
					if err := tm.client.Set(path, []byte(value)); err != nil {
						log.Printf("error in set znode %s value: %v\n", path, err)
					}
				} else {
					log.Printf("write %s/%s status to %s\n", txId, participant, StatusRolledBack)
					if err := tm.client.Set(path, []byte(StatusRolledBack)); err != nil {
						log.Printf("error in set znode %s value: %v\n", path, err)
					}
				}
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

func (tm *transactionManager) clean() {
	interval := time.Duration(time.Minute)
	for {
		exists, err := tm.client.Exists(tm.basePath)
		if err != nil || !exists {
			log.Printf("error in check base path: %v\n", err)
			time.Sleep(interval)
			continue
		}

		txTypes, err := tm.client.Children(tm.basePath)
		if err != nil {
			log.Printf("error in list base path children: %v\n", err)
			time.Sleep(interval)
			continue
		}

		for _, txType := range txTypes {
			path := tm.basePath + "/" + txType
			children, err := tm.client.Children(path)
			if err != nil {
				log.Printf("error in list transaction type %s children: %v\n", txType, err)
				continue
			}

			for _, txId := range children {
				deletable, err := tm.checkTransactionComplete(TransactionType(txType), txId)
				if err != nil {
					log.Println(err)
					continue
				}
				if deletable {
					txPath := path + "/" + txId
					if err := tm.client.DeleteRecursive(txPath); err != nil {
						log.Printf("error in delete transaction %s: %v\n", txPath, err)
					}
				}
			}
		}

		time.Sleep(interval)
	}
}

func (tm *transactionManager) checkTransactionComplete(txType TransactionType, txId string) (bool, error) {
	txPath := tm.basePath + "/" + string(txType) + "/" + txId
	log.Printf("check transaction %s\n", txPath)

	exists, err := tm.client.Exists(txPath)
	if err != nil {
		return false, fmt.Errorf("error in check path: %v", err)
	}

	if !exists {
		return false, fmt.Errorf("transaction not found")
	}

	participants, err := tm.client.Children(txPath)
	if err != nil {
		return false, fmt.Errorf("error in list transaction %s %s participants: %v", txType, txPath, err)
	}

	for _, participant := range participants {
		path := txPath + "/" + participant
		data, err := tm.client.Get(path)
		if err != nil {
			return false, fmt.Errorf("error in get znode: %v", err)
		}

		if string(data) != string(StatusCommitted) && string(data) != string(StatusRolledBack) {
			return false, nil
		}
	}

	return true, nil
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
