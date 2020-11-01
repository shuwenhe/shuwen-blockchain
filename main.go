package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"sort"

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
		ws, err := websocket.Dial(peer, "", peer)
		if err != nil {
			log.Println("dial to peer", err)
			continue
		}
		initConnection(ws)
	}
}

func initConnection(ws *websocket.Conn) {
	go wsHandleP2P(ws)
	log.Println("query lastest block.")
	ws.Write(queryLatestMsg())
}

func wsHandleP2P(ws *websocket.Conn) {
	var (
		v    = &ResponseBlockchain{}
		peer = ws.LocalAddr().String()
	)

	sockets = append(sockets, ws)
	for {
		var msg []byte
		err := websocket.Message.Receive(ws, &msg)
		if err == io.EOF {
			log.Printf("p2p Peer[%s] shutdown, remove it form peers pool.\n", peer)
			break
		}
		if err != nil {
			log.Println("Can't recevie p2p msg from", peer, err.Error())
			break
		}
		log.Printf("Received[from %s]:%s.\n", peer, msg)
		err = json.Unmarshal(msg, v)
		if err != nil {
			errFatal("invalid p2p msg", err)
		}

		switch v.Type {
		case queryLatest:
			v.Type = responseBlockchain
			bs := responseLatestMsg()
			log.Printf("reponseLaestMsg:%s\n", bs)
			ws.Write(bs)
		case queryAll:
			d, err := json.Marshal(blockChain)
			if err != nil {
				return
			}
			v.Type = responseBlockchain
			v.Data = string(d)
			bs, err := json.Marshal(v)
			if err != nil {
				return
			}
			log.Printf("responseChainMsg:%s\n", bs)
			ws.Write(bs)
		case responseBlockchain:
			handleBlockChainResponse([]byte(v.Data))
		}
	}
}

func handleBlockChainResponse(msg []byte) {
	receivedBlocks := []*Block{}
	err := json.Unmarshal(msg, &receivedBlocks)
	if err != nil {
		errFatal("invalid blockchain", err)
		return
	}
	sort.Sort(ByIndex(receivedBlocks))
	latestBlockReceived := receivedBlocks[len(receivedBlocks)-1]
	latestBlockHeld := getLatestBlock()

}

func getLatestBlock() (b *Block) {
	return blockChain[len(blockChain)-1]
}

// ResponseLatestMsg response latest msg
func responseLatestMsg() (bs []byte) {
	v := &ResponseBlockchain{
		Type: responseBlockchain,
	}
	d, _ := json.Marshal(blockChain[len(blockChain)-1:])
	v.Data = string(d)
	bs, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return bs
}

func queryLatestMsg() []byte {
	return nil
}

func main() {
	fmt.Println("shuwen-blockchain")
}
