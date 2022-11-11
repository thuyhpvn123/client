package config

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"

	ccrypto "gitlab.com/meta-node/core/crypto"
)

type Connection struct {
	Address string `json:address`
	Ip      string `json:ip`
	Port    int    `json:port`
	Type    string `json:type`
}

type Config struct {
	Address                    string `json:"address"`
	ByteAddress                []byte `json:"-"`
	BytePrivateKey             []byte `json:"-"`
	Ip                         string `json:"ip"`
	Port                       int    `json:"port"`
	NodeType                   string `json:"node_type"`
	HashPerSecond              int    `json:"hash_per_second"`
	TickPerSecond              int    `json:"tick_per_second"`
	TickPerSlot                int    `json:"tick_per_slot"`
	BlockStackSize             int    `json:"block_stack_size"`
	TimeOutTicks               int    `json:"time_out_ticks"` // how many tick validator should wait before create virture block
	TransactionPerHash         int    `json:"transaction_per_hash"`
	NumberOfValidatePohRoutine int    `json:"number_of_validate_poh_routine"`
	AccountDBPath              string `json:"account_db_path"`
	SecretKey                  string `json:"secret_key"`
	TransferFee                int    `json:"transfer_fee"`
	GuaranteeAmount            int    `json:"guarantee_amount"`

	Version          string     `json:"version"`
	BytePublicKey    []byte     `json:"-"`
	ParentConnection Connection `json:"parent_connection"`
	ServerAddress    string     `json:"server_address"`
}

func loadConfig() Config {
	var config Config
	raw, err := ioutil.ReadFile("config/conf.json")
	if err != nil {
		log.Fatalf("Error occured while reading config")
	}
	json.Unmarshal(raw, &config)
	byteAddress, err := hex.DecodeString(config.Address)
	if err != nil {
		panic("ERR when decode hex address in config")
	}
	config.ByteAddress = byteAddress
	log.Printf("Config loaded: %v\n", config)
	config.BytePrivateKey, config.BytePublicKey, config.ByteAddress = ccrypto.GenerateKeyPairFromSecretKey(config.SecretKey)
	return config
}

var AppConfig = loadConfig()

func (config Config) GetVersion() string {
	return config.Version
}

func (config Config) GetPubkey() []byte {
	return config.BytePublicKey
}

func (config Config) GetPrivateKey() []byte {
	return config.BytePrivateKey
}
