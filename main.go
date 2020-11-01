package main

import (
	"flag"
	"fmt"
	"log"

	"golang.org/x/net/websocket"
)

const (
	queryLatest = iota
	queryAll
	responseBlockchain
)

// Block struct
type Block struct {
	Index        int64  `json:"index,omitempty"`
	PreviousHash string `json:"previous_hash,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	Data         string `json:"data,omitempty"`
	Hash         string `json:"hash,omitempty"`
}

// Initialize structure
var genesisBlock = &Block{
	Index:        0,
	PreviousHash: "0",
	Timestamp:    1604190255,
	Data:         "my genesis block",
	Hash:         "966634ebf2fc135707d6753692bf4b1e",
}

var (
	sockets      []*websocket.Conn
	blockChain   = []*Block{genesisBlock}
	httpPort     = flag.String("api", "3001", "Api server port!")
	p2pPort      = flag.String("p2p", ":6001", "p2p server port!")
	InitialPeers = flag.String("peers", "ws://localhost:6001", "initial peers!")
)

func (b *Block) String() string {
	return fmt.Sprintf("index:%d,previousHash:%s,timestamp:%d,data:%s,hash:%s", b.Index, b.PreviousHash, b.Timestamp, b.Data, b.Hash)
}

type ByIndex []*Block

func (b ByIndex) Len() int {
	return len(b)
}

func (b ByIndex) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b ByIndex) Less(i, j int) bool {
	return b[i].Index < b[j].Index
}

// ResponseBlockchain struct
type ResponseBlockchain struct {
	Type int    `json:"type,omitempty"`
	Data string `json:"data,omitempty"`
}

func errFatal(msg string, err error) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}

func connectToPeers(peersPort []string) { // p2p
	for _, peer := range peersPort {
		if peer == "" {
			continue
		}
		websocket.Dial(peer, "", peer)

	}
}

func main() {
	fmt.Println("shuwen-blockchain")
}
