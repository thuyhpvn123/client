package network

import (
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	"gitlab.com/meta-node/client/config"
	cn "gitlab.com/meta-node/core/network"
)

type Server struct {
	Address        string
	IP             string
	Port           int
	MessageHandler *MessageHandler
}

func (server *Server) ConnectToParent(connection *cn.Connection) {
	err := connection.Connect()
	if err != nil {
		log.Warn("Error when connect to %v:%v, wallet adress : %v", err)
	} else {
		server.MessageHandler.OnConnect(connection, server.Address)
		go server.MessageHandler.HandleConnection(connection)
	}
}

func ConnectToServer(serverConnectionStr string) *cn.Connection {
	split := strings.Split(serverConnectionStr, ":")
	port, _ := strconv.Atoi(split[1])
	conn := cn.NewConnection(common.Address{}, split[0], port, "")
	err := conn.Connect()

	if err != nil {
		panic(err)
	}

	messageHandler := MessageHandler{
		config: &config.AppConfig,
	}
	go messageHandler.HandleConnection(conn)
	return conn
}
