package transaction

import "time"

type ResourceType string
type TransactionType string
type TransactionStatus string

const (
	OrderCreation TransactionType = "ORDER_CREATION"

	OrderResource ResourceType = "ORDER_RESOURCE"
	UserResource  ResourceType = "USER_RESOURCE"
)

const (
	StatusInit       TransactionStatus = "INIT"
	StatusPrepared   TransactionStatus = "PREPARED"
	StatusReady      TransactionStatus = "READY"
	StatusAbort      TransactionStatus = "ABORT"
	StatusCommit     TransactionStatus = "COMMIT"
	StatusCommitted  TransactionStatus = "COMMITTED"
	StatusRollBack   TransactionStatus = "ROLL_BACK"
	StatusRolledBack TransactionStatus = "ROLLED_BACK"
)

var (
	TransactionTypes []TransactionType = []TransactionType{
		OrderCreation,
	}
	ResourceTypes []ResourceType = []ResourceType{
		OrderResource,
		UserResource,
	}
)

type TransactionData struct {
	Id           string            `json:"id"`
	Type         TransactionType   `json:"type"`
	Timestamp    time.Time         `json:"timestamp"`
	Payload      []byte            `json:"payload"`
	Status       TransactionStatus `json:"status"`
	Participants []string          `json:"participants"`
}

type TransactionHandler func(txData TransactionData) error
type TransactionFinalizeHandler func(txId string) error

type TransactionWatcher interface {
	RegisterHandler(TransactionType, TransactionHandler, TransactionFinalizeHandler)
	GetBasePath() string
	Watch()
	Stop()
}

type TransactionManager interface {
	Begin(txType TransactionType, data []byte, participants []string, resources []ResourceType) (string, error)
	Prepare(txId string) error
	Finalize(txId string, isCommit bool) error
	GetVotesResult(txId string) (bool, error)
}
