package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/synternet/data-layer-sdk/pkg/options"
	"github.com/synternet/data-layer-sdk/pkg/service"

	"github.com/synternet/ethereum-publisher/pkg/ipc"
)

type SubjectConstants struct {
	StreamedHeader     string
	StreamedBlock      string
	StreamedTx         string
	StreamedTxLogEvent string
	StreamedTxMemPool  string
	SteramedTraceCall  string
}

func NewSubjectConstants() *SubjectConstants {
	return &SubjectConstants{
		StreamedHeader:     "header",
		StreamedBlock:      "block",
		StreamedTx:         "tx",
		StreamedTxLogEvent: "log-event",
		StreamedTxMemPool:  "mempool",
		SteramedTraceCall:  "trace_call",
	}
}

type Service struct {
	*service.Service
	ipc      *ipc.Ipc
	network  string
	subjects *SubjectConstants
	headers  chan *types.Header
	logs     chan types.Log
	ErrCh    chan error
}

func New(ipcSvc *ipc.Ipc, opts ...options.Option) *Service {
	svc := &Service{
		Service:  &service.Service{},
		ipc:      ipcSvc,
		subjects: NewSubjectConstants(),
		headers:  make(chan *types.Header),
		logs:     make(chan types.Log),
		ErrCh:    make(chan error),
	}

	err := svc.Configure(opts...)
	if err != nil {
		panic(err)
	}

	return svc
}

func (s *Service) Start() context.Context {
	s.Run()
	return s.Service.Start()
}

func (s *Service) run() {
	defer close(s.ErrCh)
	go s.subscribeNewHeaders()
	go s.subscribeNewLog()
	go s.ipc.MonitorPendingTransactions()

	for {
		select {
		case <-s.Context.Done():
			return
		case txPoolMessages := <-s.ipc.TxMessagesCh:
			err := s.PublishBuf(txPoolMessages.AsJSON(), s.subjects.StreamedTxMemPool)
			if err != nil {
				log.Println(err)
			}
		case txTransactionBlock := <-s.ipc.TxMessageBlock:
			err := s.PublishBuf(txTransactionBlock.AsJSON(), s.subjects.StreamedTx)
			if err != nil {
				log.Println(err)
			}
		case txTraceCall := <-s.ipc.TxTraceCalls:
			err := s.PublishBuf(txTraceCall.AsJSON(), s.subjects.SteramedTraceCall)
			if err != nil {
				log.Println(err)
			}
		case ipcError := <-s.ipc.IpcErrorCh:
			log.Println(ipcError)
		}
	}
}

func (s *Service) subscribeNewHeaders() {
	subHeaders, err := s.ipc.EthClient.SubscribeNewHead(s.Context, s.headers)
	if err != nil {
		s.ErrCh <- err

		return
	}
	log.Println("subscribed to newHeader")
	for {
		select {
		case errHeaders := <-subHeaders.Err():
			s.ErrCh <- errHeaders
		case header := <-s.headers:
			rhead, rblock, err := s.ipc.ProcessAndPrepareBlock(header)
			if err != nil {
				log.Printf("Processing header data: %s", err.Error())

				continue
			}

			log.Printf("Captured Head: %s\r\n", header.Hash().Hex())

			rheadBytes, err := json.Marshal(rhead)
			if err != nil {
				log.Printf("Marshalling header data: %s", err.Error())
				continue
			}
			err = s.PublishBuf(rheadBytes, s.subjects.StreamedHeader)
			if err != nil {
				log.Println(err)
			}

			rblockBytes, err := json.Marshal(rblock)
			if err != nil {
				log.Printf("Marshalling block data: %s", err.Error())
				continue
			}
			err = s.PublishBuf(rblockBytes, s.subjects.StreamedBlock)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (s *Service) subscribeNewLog() {
	filter := ethereum.FilterQuery{
		Addresses: []common.Address{}, // filter all addresses
	}
	subTxPool, err := s.ipc.EthClient.SubscribeFilterLogs(s.Context, filter, s.logs)
	if err != nil {
		s.ErrCh <- err
		return
	}
	log.Println("subscribed to newLog")
	for {
		select {
		case errTxPool := <-subTxPool.Err():
			s.ErrCh <- errTxPool
		case txLog := <-s.logs:
			if txLog.Removed { // Transaction is no longer in tx-pool log
				continue
			}
			txLogBytes, err := json.Marshal(txLog)
			if err != nil {
				log.Printf("Marshalling tx-log data: %s", err.Error())
				continue
			}
			errLog := s.PublishBuf(txLogBytes, s.subjects.StreamedTxLogEvent)
			if errLog != nil {
				s.ErrCh <- errLog
				continue
			}
			fmt.Println("log message for transaction:", txLog.TxHash.Hex())
		}
	}
}

func (s *Service) Run() <-chan error {
	go s.run()

	return s.ErrCh
}
