package ipc

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	lru "github.com/hashicorp/golang-lru/v2"

	rtypes "github.com/synternet/ethereum-publisher/pkg/types"
)

type Ipc struct {
	ctx            context.Context
	TxMessagesCh   chan rtypes.PendingTransactionInPool
	TxMessagesDup  *lru.Cache[common.Hash, bool]
	TxMessageBlock chan rtypes.Transaction
	TxTraceCalls   chan rtypes.TraceCallTransactionResponse
	IpcErrorCh     chan error
	IpcPath        string
	EthClient      *ethclient.Client
	RpcClient      *rpc.Client
}

func NewClient(ctx context.Context, ipcPath string) (*Ipc, error) {
	ret := &Ipc{
		ctx:            ctx,
		IpcPath:        ipcPath,
		TxMessagesCh:   make(chan rtypes.PendingTransactionInPool, 1000),
		TxMessageBlock: make(chan rtypes.Transaction, 1000),
		TxTraceCalls:   make(chan rtypes.TraceCallTransactionResponse, 1000),
		IpcErrorCh:     make(chan error, 1000),
	}

	lcache, err := lru.New[common.Hash, bool](20000)
	if err != nil {
		return nil, err
	}

	ethClient, err := ethclient.Dial(ipcPath)
	if err != nil {
		return nil, err
	}

	rpcClient, err := rpc.Dial(ipcPath)
	if err != nil {
		return nil, err
	}

	ret.EthClient = ethClient
	ret.RpcClient = rpcClient
	ret.TxMessagesDup = lcache

	return ret, nil
}

func (c *Ipc) GetChainID() (*big.Int, error) {
	return c.EthClient.NetworkID(c.ctx)
}

func (c *Ipc) Close() error {
	if c.EthClient != nil {
		c.EthClient.Close()
	}

	return nil
}

func (c *Ipc) ProcessAndPrepareBlock(header *types.Header) (rHeader *types.Header, rBlock *rtypes.Block, err error) {
	block, err := c.EthClient.BlockByHash(c.ctx, header.Hash())
	if err != nil {
		return
	}

	rBlock = &rtypes.Block{
		Hash:              block.Hash().Hex(),
		Number:            block.Number().Uint64(),
		Time:              block.Time(),
		Nonce:             block.Nonce(),
		TransactionsIds:   c.processTransactionIdsFromBlock(block),
		TransactionsCount: len(block.Transactions()),
	}

	go c.ProcessAndPrepareTransactions(block)
	go c.processTraceCallFromTx(block)

	return header, rBlock, nil
}

func (c *Ipc) processTransactionIdsFromBlock(block *types.Block) []string {
	transactionIdsArray := []string{}

	if len(block.Transactions()) == 0 {
		return transactionIdsArray
	}

	for _, tx := range block.Transactions() {
		transactionIdsArray = append(transactionIdsArray, tx.Hash().Hex())
	}

	return transactionIdsArray
}

func (c *Ipc) ProcessAndPrepareTransactions(block *types.Block) {
	if len(block.Transactions()) == 0 {
		return
	}

	log.Printf("transactions count in block: %d", len(block.Transactions()))

	for _, tx := range block.Transactions() {
		// extract V,R,S signature values from transaction
		txv, txr, txs := tx.RawSignatureValues()

		txRet := rtypes.Transaction{
			BlockHash: block.Hash().Hex(),
			Hash:      tx.Hash().Hex(),
			Value:     tx.Value().String(),
			Gas:       tx.Gas(),
			GasPrice:  tx.GasPrice().Uint64(),
			V:         txv,
			R:         txr,
			S:         txs,
			Nonce:     tx.Nonce(),
			Input:     common.Bytes2Hex(tx.Data()),
			Timestamp: time.Unix(int64(block.Time()), 0),
		}
		if tx.To() != nil {
			txRet.To = tx.To().Hex()
		}
		if msg, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
			txRet.From = msg.Hex()
		}

		c.TxMessageBlock <- txRet
	}
}

func (c *Ipc) GetPendingTxdetailsByHash(txLog types.Log) (rtypes.TransactionInPool, error) {
	var rtx rtypes.TransactionInPool
	receipt, err := c.EthClient.TransactionReceipt(c.ctx, txLog.TxHash)
	if err != nil {
		return rtx, err
	}
	tx, _, err := c.EthClient.TransactionByHash(c.ctx, receipt.TxHash)
	if err != nil {
		return rtx, err
	}
	block, err := c.EthClient.BlockByHash(c.ctx, receipt.BlockHash)
	if err != nil {
		return rtx, err
	}
	txv, txr, txs := tx.RawSignatureValues()
	rtx = rtypes.TransactionInPool{
		Hash:             tx.Hash().Hex(),
		Type:             receipt.Type,
		Value:            tx.Value().String(),
		Gas:              tx.Gas(),
		GasPrice:         tx.GasPrice().Uint64(),
		V:                txv,
		R:                txr,
		S:                txs,
		Nonce:            tx.Nonce(),
		To:               tx.To().Hex(),
		Input:            common.Bytes2Hex(tx.Data()),
		BlockHash:        receipt.BlockHash,
		BlockNumber:      receipt.BlockNumber,
		TransactionIndex: receipt.TransactionIndex,
		Timestamp:        time.Unix(int64(block.Time()), 0),
	}

	if from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
		rtx.From = from.Hex()
	}

	return rtx, nil
}

func (c *Ipc) GetCheckToAddress(tx *types.Transaction) string {
	var toAddress string
	if tx.To() != nil {
		toAddress = tx.To().Hex()
	} else {
		toAddress = "Contract Creation"
	}
	return toAddress
}

func (c *Ipc) MonitorPendingTransactions() {
	newTxsChannel := make(chan common.Hash)
	sub, err := c.RpcClient.EthSubscribe(c.ctx, newTxsChannel, "newPendingTransactions")
	defer sub.Unsubscribe()

	if err != nil {
		c.IpcErrorCh <- fmt.Errorf("error while subscribing: %s", err.Error())
	}

	log.Println("subscribed to txpool transactions")

	for {
		select {
		case err := <-sub.Err():
			panic(fmt.Errorf("newPendingTransactions subscription error: %w", err))
		// Code block is executed when a new tx hash is piped to the channel
		case transactionHash := <-newTxsChannel:
			// Check duplication cache for tx hash and skip if already processed
			if !c.TxMessagesDup.Contains(transactionHash) {
				c.TxMessagesDup.Add(transactionHash, true)
			} else {
				c.IpcErrorCh <- fmt.Errorf("transaction: %s already in cache. cache count: %d", transactionHash, c.TxMessagesDup.Len())
				continue
			}
			// Get transaction object from hash by querying the client
			tx, is_pending, _ := c.EthClient.TransactionByHash(context.Background(), transactionHash)
			// If tx is valid and still unconfirmed
			if is_pending {
				// Skip Temporary inconsistency or Re-orgs and dropped transactions
				if tx != nil {
					from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
					if err != nil {
						c.IpcErrorCh <- fmt.Errorf("can't extract address from for %s with: %s", tx.Hash(), err.Error())
					}
					// extract V,R,S signature values from transaction
					var rtx rtypes.PendingTransactionInPool
					txv, txr, txs := tx.RawSignatureValues()

					rtx = rtypes.PendingTransactionInPool{
						Hash:     tx.Hash().Hex(),
						Value:    tx.Value().String(),
						Gas:      tx.Gas(),
						GasPrice: tx.GasPrice().Uint64(),
						V:        txv,
						R:        txr,
						S:        txs,
						Nonce:    tx.Nonce(),
						To:       c.GetCheckToAddress(tx),
						From:     from.Hex(),
						Input:    common.Bytes2Hex(tx.Data()),
					}
					c.TxMessagesCh <- rtx
				} else {
					c.IpcErrorCh <- fmt.Errorf("received a nil transaction object for %s", tx.Hash())
				}
			}
		}
	}
}

func (c *Ipc) processTraceCallFromTx(block *types.Block) {
	var txHashes []string
	for _, tx := range block.Transactions() {
		txHashes = append(txHashes, tx.Hash().Hex())
	}
	c.processTraceCallDetails(txHashes)
}

func (c *Ipc) processTraceCallDetails(txHashes []string) {
	// Prepare the trace configuration options
	traceConfig := map[string]interface{}{
		//"disableMemory": true,
		//"disableStack":  true,
		"tracer": "callTracer",
	}

	// Create a slice to hold the batch elements
	var batch []rpc.BatchElem
	var results []*rtypes.TraceCallTransactionResponse

	// Add the requests to the batch
	for _, txHash := range txHashes {
		result := new(rtypes.TraceCallTransactionResponse)
		results = append(results, result)
		batch = append(batch, rpc.BatchElem{
			Method: "debug_traceTransaction",
			Args:   []interface{}{txHash, traceConfig},
			Result: result,
		})
	}

	// Send the batched requests
	err := c.RpcClient.BatchCallContext(c.ctx, batch)
	if err != nil {
		log.Fatal(fmt.Errorf("error while calling BatchCallContext: %w", err))
	}

	// Process the responses
	for i, elem := range batch {
		if elem.Error != nil {
			log.Fatal(elem.Error)
		}

		results[i].TxHash = txHashes[i]
		c.TxTraceCalls <- *results[i]
	}
}
