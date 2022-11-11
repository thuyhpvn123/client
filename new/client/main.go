package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb"
	"gitlab.com/meta-node/client/cli"
	"gitlab.com/meta-node/client/config"
	"gitlab.com/meta-node/client/network"
	server_app "gitlab.com/meta-node/client/server/app"
	core_crypto "gitlab.com/meta-node/core/crypto"
	cn "gitlab.com/meta-node/core/network"
	pb "gitlab.com/meta-node/core/proto"
)

func initParentConnections() *cn.Connection {
	parentConfig := config.AppConfig.ParentConnection
	bAddress, _ := hex.DecodeString(parentConfig.Address)
	parentConnection := cn.NewConnection(
		common.BytesToAddress(bAddress),
		parentConfig.Ip,
		parentConfig.Port,
		parentConfig.Type,
	)
	return parentConnection
}

func main() {
	core_crypto.Init()
	log.SetLevel(log.DebugLevel)
	finish := make(chan bool)
	length := make([]byte, 8)
	binary.LittleEndian.PutUint64(length, uint64(500))
	// db
	fmt.Printf(config.AppConfig.AccountDBPath)

	accountDB, err := leveldb.OpenFile(config.AppConfig.AccountDBPath, nil)
	if err != nil {
		panic(err)
	}
	defer accountDB.Close()

	accountStateChan := make(chan *pb.AccountState)
	receiptChan := make(chan *pb.Receipt)
	transactionChan := make(chan *pb.Transaction)
	parentConn := initParentConnections()
	// api server
	app := server_app.InitApp(&config.AppConfig, parentConn, accountStateChan, receiptChan, transactionChan)
	go app.Run()
	runServer(accountDB, parentConn, accountStateChan, receiptChan, transactionChan)

	<-finish
	fmt.Printf("Main\n")
}

func runServer(
	accountDB *leveldb.DB,
	parentConn *cn.Connection,
	accountStateChan chan *pb.AccountState,
	receiptChan chan *pb.Receipt,
	transactionChan chan *pb.Transaction,
) *network.Server {
	chData := make(chan interface{})
	messageHandler := network.NewMessageHandler(&config.AppConfig, accountStateChan, receiptChan, transactionChan)
	messageHandler.SetChan(chData)
	server := network.Server{
		Address:        config.AppConfig.Address,
		IP:             config.AppConfig.Ip,
		Port:           config.AppConfig.Port,
		MessageHandler: messageHandler,
	}
	server.ConnectToParent(parentConn)
	cli.StartCli(config.AppConfig, parentConn, chData, &server)
	return &server
}
