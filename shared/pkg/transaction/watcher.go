package transaction

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/zkclient"
)

type transactionWatcher struct {
	client           *zkclient.ZooKeeperClient
	basePath         string
	handlers         map[TransactionType]TransactionHandler
	finalizeHandlers map[TransactionType]TransactionFinalizeHandler
	stopChan         chan struct{}
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

func NewTransactionWatcher(client *zkclient.ZooKeeperClient) (*transactionWatcher, error) {
	tw := &transactionWatcher{
		client:           client,
		basePath:         "/transactions",
		handlers:         make(map[TransactionType]TransactionHandler),
		finalizeHandlers: make(map[TransactionType]TransactionFinalizeHandler),
		stopChan:         make(chan struct{}),
	}

	if err := tw.init(); err != nil {
		return nil, err
	}

	return tw, nil
}

func (tw *transactionWatcher) RegisterHandler(txType TransactionType,
	handler TransactionHandler, finalizeHandler TransactionFinalizeHandler) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.handlers[txType] = handler
	tw.finalizeHandlers[txType] = finalizeHandler
}

func (tw *transactionWatcher) GetBasePath() string {
	return tw.basePath
}

func (tw *transactionWatcher) Watch() {
	for txType := range tw.handlers {
		tw.wg.Add(1)
		go tw.watchTransaction(txType)
	}
}

func (tw *transactionWatcher) Stop() {
	close(tw.stopChan)
	tw.wg.Wait()
}

func (tw *transactionWatcher) init() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for {
		exists, ch, err := tw.client.ExistsW(tw.basePath)
		if err != nil {
			return fmt.Errorf("error in check base znode %s: %v", tw.basePath, err)
		}

		if exists {
			for _, txType := range TransactionTypes {
				for {
					path := tw.basePath + "/" + string(txType)

					exists, txCh, err := tw.client.ExistsW(path)
					if err != nil {
						return fmt.Errorf("error in check znode %s: %v", path, err)
					}

					if exists {
						break
					}

					<-txCh
				}
			}
			break
		}

		<-ch
	}

	return nil
}

func (tw *transactionWatcher) watchTransaction(txType TransactionType) {
	defer tw.wg.Done()
	for {
		select {
		case <-tw.stopChan:
			return
		default:
			path := tw.basePath + "/" + string(txType)
			children, ch, err := tw.client.ChildrenW(path)
			if err != nil {
				log.Printf("watch %s children failed: %v\n", txType, err)
				continue
			}

			for _, child := range children {
				if err := tw.processTransaction(txType, child); err != nil {
					log.Printf("process %s transaction failed: %v\n", txType, err)
					continue
				}
			}

			select {
			case <-tw.stopChan:
				return
			case <-ch:
			}
		}
	}
}

func (tw *transactionWatcher) processTransaction(txType TransactionType, txId string) error {
	path := tw.basePath + "/" + string(txType) + "/" + txId
	data, err := tw.client.Get(path)
	if err != nil {
		return fmt.Errorf("error getting transaction %s %s data: %v", txType, txId, err)
	}

	// unmarshal txData
	var txData TransactionData
	if err := json.Unmarshal(data, &txData); err != nil {
		return fmt.Errorf("error unmarshaling transaction data: %v", err)
	}

	// get handler and execute
	tw.mu.RLock()
	handler, handlerExists := tw.handlers[txType]
	finalizeHandler, finalizeHandlerExists := tw.finalizeHandlers[txType]
	tw.mu.RUnlock()

	if !handlerExists || !finalizeHandlerExists {
		return fmt.Errorf("%s handler not exists", txType)
	}

	if err := handler(txData); err != nil {
		return err
	}

	if err := finalizeHandler(txData.Id); err != nil {
		return err
	}

	return nil
}
