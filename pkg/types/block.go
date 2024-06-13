package types

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Block struct {
	Hash              string   `json:"blockHash"`
	Number            uint64   `json:"blockNumber"`
	Time              uint64   `json:"time"`
	Nonce             uint64   `json:"nonce"`
	TransactionsIds   []string `json:"transactionIds"`
	TransactionsCount int      `json:"transactionsCount"`
}

// Transaction represent Eth TX structure for JSON response
type Transaction struct {
	BlockHash string    `json:"blockHash"`
	Hash      string    `json:"hash"`
	Value     string    `json:"value"`
	Gas       uint64    `json:"gas"`
	GasPrice  uint64    `json:"gasPrice"`
	V         *big.Int  `json:"v"`
	R         *big.Int  `json:"r"`
	S         *big.Int  `json:"s"`
	Nonce     uint64    `json:"nonce"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Input     string    `json:"input"`
	Timestamp time.Time `json:"timeStamp"`
}

type TransactionInPool struct {
	Hash             string      `json:"hash"`
	Type             uint8       `json:"type"`
	Value            string      `json:"value"`
	Gas              uint64      `json:"gas"`
	GasPrice         uint64      `json:"gasPrice"`
	V                *big.Int    `json:"v"`
	R                *big.Int    `json:"r"`
	S                *big.Int    `json:"s"`
	Nonce            uint64      `json:"nonce"`
	From             string      `json:"from"`
	To               string      `json:"to"`
	TransactionIndex uint        `json:"transactionIndex"`
	BlockHash        common.Hash `json:"blockHash,omitempty"`
	BlockNumber      *big.Int    `json:"blockNumber,omitempty"`
	Timestamp        time.Time   `json:"timeStamp"`
	Input            string      `json:"input"`
}

type PendingTransactionInPool struct {
	Hash     string   `json:"hash"`
	Value    string   `json:"value"`
	Gas      uint64   `json:"gas"`
	GasPrice uint64   `json:"gasPrice"`
	V        *big.Int `json:"v"`
	R        *big.Int `json:"r"`
	S        *big.Int `json:"s"`
	Nonce    uint64   `json:"nonce"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Input    string   `json:"input"`
}

func (t *Transaction) AsJSON() []byte {
	json, err := json.Marshal(t)
	if err != nil {
		return []byte{}
	}
	return json
}

func (pt *PendingTransactionInPool) AsJSON() []byte {
	json, err := json.Marshal(pt)
	if err != nil {
		return []byte{}
	}
	return json
}
