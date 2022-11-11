package network

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"

	log "github.com/sirupsen/logrus"
	"gitlab.com/meta-node/client/config"
	"gitlab.com/meta-node/client/network/messages"
	"gitlab.com/meta-node/client/transactionsDB"
	cc "gitlab.com/meta-node/core/controllers"
	cn "gitlab.com/meta-node/core/network"
	pb "gitlab.com/meta-node/core/proto"
)

var chData chan interface{}

type MessageHandler struct {
	config           *config.Config
	accountStateChan chan *pb.AccountState
	receiptChan      chan *pb.Receipt
	transactionChan  chan *pb.Transaction
}

func NewMessageHandler(
	config *config.Config,
	accountStateChan chan *pb.AccountState,
	receiptChan chan *pb.Receipt,
	transactionChan chan *pb.Transaction,
) *MessageHandler {
	return &MessageHandler{
		config,
		accountStateChan,
		receiptChan,
		transactionChan,
	}
}

func (handler *MessageHandler) SetChan(ch chan interface{}) {
	chData = ch
}

func (handler *MessageHandler) OnConnect(conn *cn.Connection, address string) {
	log.Info(fmt.Sprintf("OnConnect with server %s", conn.GetTcpConnection().RemoteAddr()))
	cn.SendMessage(handler.config, conn, messages.InitConnection, &pb.InitConnection{
		Address: common.FromHex(address),
		Type:    "Client",
	})
}

func (handler *MessageHandler) OnDisconnect(conn *cn.Connection) {
	conn.Close()
	log.Info(fmt.Printf("Disconnected with server  %s, wallet address: %v", conn.GetTcpConnection().RemoteAddr(), conn.GetAddress()))
}

func (h *MessageHandler) HandleConnection(conn *cn.Connection) {
	defer h.OnDisconnect(conn)
	for {
		bLength := make([]byte, 8)
		_, err := io.ReadFull(conn.GetTcpConnection(), bLength)
		if err != nil {
			switch err {
			case io.EOF:
				return
			default:
				log.Errorf("server error: %v\n", err)
				return
			}
		}
		messageLength := binary.LittleEndian.Uint64(bLength)

		maxMsgLength := uint64(1073741824)
		if messageLength > maxMsgLength {
			log.Errorf("Invalid messageLength want less than %v, receive %v\n", maxMsgLength, messageLength)
			return
		}

		data := make([]byte, messageLength)
		byteRead, err := io.ReadFull(conn.GetTcpConnection(), data)
		if err != nil {
			switch err {
			case io.EOF:
				return
			default:
				log.Errorf("server error: %v\n", err)
				return
			}
		}

		if uint64(byteRead) != messageLength {
			log.Errorf("Invalid message receive byteRead !=  messageLength %v, %v\n", byteRead, messageLength)
			return
		}

		message := pb.Message{}

		err = proto.Unmarshal(data[:messageLength], &message)
		if err == nil {
			h.ProcessMessage(conn, &message)
		}
	}
}

func (handler *MessageHandler) ProcessMessage(conn *cn.Connection, message *pb.Message) {
	switch message.Header.Command {
	case "InitConnection":
		handler.handleInitConnectionMessage(conn, message)
	case "ConfirmedTransaction":
		handler.handleConfirmedTransaction(conn, message)
	case "AccountState":
		handler.handlerAccountState(message)
	case "MinerGetSmartContractStateResult":
		handler.handlerSmartContractState(conn, message)
	case "TransactionError":
		handler.handlerTransactionError(message)
	case "Receipt":
		handler.handleReceipt(conn, message)
	case "NewLogs":
		handler.handleNewLogs(message)
	case "QueryLogsResult":
		handler.handleQueryLogsResult(message)
	case "GetTransactionResult":
		handler.handleGetTransactionResult(message)
	default:
		log.Warnf("Receive invalid message %v\n", message.Header.Command)
	}
}

func (handler *MessageHandler) handleReceipt(conn *cn.Connection, message *pb.Message) {
	log.Info("Receive Receipt from", conn.GetTcpConnection().RemoteAddr())
	receipt := &pb.Receipt{}
	select {
	case handler.receiptChan <- receipt:
	default:
	}
	proto.Unmarshal(message.Body, receipt)
	log.Infof("Receipt: \nTransaction hash %v\nFrom %v\nTo %v\nAmount %v\nStatus %v\nReturn %v\n",
		common.BytesToHash(receipt.TransactionHash),
		common.BytesToAddress(receipt.FromAddress),
		common.BytesToAddress(receipt.ToAddress),
		uint256.NewInt(0).SetBytes(receipt.Amount),
		receipt.Status,
		common.Bytes2Hex(receipt.ReturnValue),
	)
}

func (handler *MessageHandler) handleConfirmedTransaction(conn *cn.Connection, message *pb.Message) {
	log.Info("Receive handleConfirmedTransaction from", conn.GetTcpConnection().RemoteAddr())
	transaction := &pb.Transaction{}
	// save transaction to file
	bData, _ := proto.Marshal(transaction)
	err := os.WriteFile("./data_confirmed", bData, 0644)
	if err != nil {
		log.Fatalf("Error when write data %v", err)
	}
	proto.Unmarshal(message.Body, transaction)
	log.Infoln(transaction)

	transactionsDb := transactionsDB.GetInstanceTransactionsDB()
	if common.BytesToHash(transactionsDb.PendingTransaction.Hash) == common.BytesToHash(transaction.Hash) {
		transactionsDb.SavePendingTransaction()
	} else {
		log.Warn("PendingTransaction not match")
	}
}

func (handler *MessageHandler) handleInitConnectionMessage(conn *cn.Connection, message *pb.Message) {
	log.Info("Receive InitConnection from", conn.GetTcpConnection().RemoteAddr())
}

func (handler *MessageHandler) handlerAccountState(message *pb.Message) {

	if len(message.Body) == 0 {
		log.Info("Account have no data")
		as := &pb.AccountState{
			PendingBalance: []byte{},
			Balance:        []byte{},
			LastHash:       cc.GetEmptyTransaction().Hash,
		}
		select {
		case chData <- as:
		default:
		}
		select {
		case handler.accountStateChan <- as:
		default:
		}
		return
	} else {
		accountState := &pb.AccountState{}
		proto.Unmarshal(message.Body, accountState)
		select {
		case chData <- accountState:
			return
		default:
		}
		log.Infof(`
			Account data: 
			Address: %v 
			lastHash:%v 
			Balance: %v 
			Pending Balance: %v 
			SmartContractInfo: %v`,
			hex.EncodeToString(accountState.Address),
			hex.EncodeToString(accountState.LastHash),
			uint256.NewInt(0).SetBytes(accountState.Balance),
			uint256.NewInt(0).SetBytes(accountState.PendingBalance),
			accountState.SmartContractInfo,
		)
		select {
		case handler.accountStateChan <- accountState:
		default:
		}
		// connect to storage to get smart contract state

		if accountState.SmartContractInfo != nil {
			split := strings.Split(accountState.SmartContractInfo.StorageHost, ":")
			port, _ := strconv.Atoi(split[1])
			conn := cn.NewConnection(common.Address{}, split[0], port, "")
			err := conn.Connect()
			if err == nil {
				cn.SendBytes(handler.config, conn, messages.MinerGetSmartContractState, accountState.Address)
				go handler.HandleConnection(conn)
			} else {
				fmt.Print(fmt.Errorf("err when connect to storage host %v", err))
			}
		}

	}

}

func (handler *MessageHandler) handlerSmartContractState(conn *cn.Connection, message *pb.Message) {
	log.Info("SmartContractState: ")
	if len(message.Body) == 0 {
		log.Info("Account have no data")
		return
	} else {
		rs := &pb.SmartContractStateResult{}
		fmt.Println("storage: %v", rs)

		proto.Unmarshal(message.Body, rs)
		fmt.Printf("code: %v\n", hex.EncodeToString(rs.SmartContractState.Code))
		fmt.Println("storage:")
		for i, v := range rs.SmartContractState.Storage {
			fmt.Printf("%v:%v\n", i, common.Bytes2Hex(v))
		}
		conn.Close()
	}
}

func (handler *MessageHandler) handlerTransactionError(message *pb.Message) {
	err := &pb.Error{}
	proto.Unmarshal(message.Body, err)
	fmt.Printf("handlerTransactionError: %v\n", err)
}

func (handler *MessageHandler) handleNewLogs(message *pb.Message) {
	logs := &pb.Logs{}
	proto.Unmarshal(message.Body, logs)
	fmt.Println("========== Receive NewLogs: =========== ")
	for _, log := range logs.Logs {
		fmt.Printf("Address: %v\nData: %v\n", common.Bytes2Hex(log.Address), common.Bytes2Hex(log.Data))
		for i, t := range log.Topics {
			fmt.Printf("Topic %v: %v\n", i, common.Bytes2Hex(t))
		}
	}
}

func (handler *MessageHandler) handleQueryLogsResult(message *pb.Message) {
	logs := &pb.Logs{}
	proto.Unmarshal(message.Body, logs)
	fmt.Println("========== Receive Query logs result: =========== ")
	for _, log := range logs.Logs {
		fmt.Printf("Address: %v\nData: %v\n", common.Bytes2Hex(log.Address), common.Bytes2Hex(log.Data))
		for i, t := range log.Topics {
			fmt.Printf("Topic %v: %v\n", i, common.Bytes2Hex(t))
		}
	}
}

func (handler *MessageHandler) handleGetTransactionResult(message *pb.Message) {
	transaction := &pb.Transaction{}
	proto.Unmarshal(message.Body, transaction)
	fmt.Println("========== Receive Get Transaction Result: =========== ")
	fmt.Printf("From: %v\nTo: %v\nAmount: %v\nHash: %v\n",
		common.Bytes2Hex(transaction.FromAddress),
		common.Bytes2Hex(transaction.ToAddress),
		common.Bytes2Hex(transaction.Amount),
		common.Bytes2Hex(transaction.Hash),
	)
	select {
	case handler.transactionChan <- transaction:
	default:
	}
}
