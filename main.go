package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

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
	Timestamp    int64  `json:"timestamp,omitempty"` // Each block is nested in chronological order
	Data         string `json:"data,omitempty"`
	Hash         string `json:"hash,omitempty"`
}

// Initialize structure
var genesisBlock = &Block{
	Index:        0,
	PreviousHash: "0",
	Timestamp:    1604190255,         // 每个区块按照时间顺序层层嵌套
	Data:         "my genesis block", // Genesis Block is the first block in the blockChain
	Hash:         "966634ebf2fc135707d6753692bf4b1e",
}

var (
	sockets      []*websocket.Conn
	blockChain   = []*Block{genesisBlock} // Block Chain
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

type ResponseBlockchain struct { // ResponseBlockchain struct
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
	latestBlockReceived := receivedBlocks[len(receivedBlocks)-1] // Lastest received blocks
	latestBlockHeld := getLatestBlock()
	if latestBlockReceived.Index > latestBlockHeld.Index {
		log.Printf("blockchain possibly behind. We got:%d Peer got:%d", latestBlockHeld.Index, latestBlockReceived.Index)
		if latestBlockHeld.Hash == latestBlockReceived.PreviousHash {
			log.Println("We can append the received block to our chain")
			blockChain = append(blockChain, latestBlockReceived)
		} else if len(receivedBlocks) == 1 {
			log.Println("We have to query the chain from our peer.")
			broadcast(queryAllMsg())
		} else {
			log.Println("Received blockchain is longer than current blockchain!")
			replaceChain(receivedBlocks)
		}
	} else {
		log.Println("Received blockchain is not longer than current blockchain!")
	}
}

func calculateHashForBlock(b *Block) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%s%d%s", b.Index, b.PreviousHash, b.Timestamp, b.Data))))
}

func isValidNewBlock(nb, pb *Block) (ok bool) {
	if nb.Hash == calculateHashForBlock(nb) && pb.Index+1 == nb.Index && pb.Hash == nb.PreviousHash {
		ok = true
	}
	return
}

func isValidChain(bc []*Block) bool {
	if bc[0].String() != genesisBlock.String() {
		log.Println("No same GenesisBlock.", bc[0].String())
		return false
	}
	temp := []*Block{bc[0]}
	for i := 1; i < len(bc); i++ {
		if isValidNewBlock(bc[i], temp[i-1]) {
			temp = append(temp, bc[i])
		} else {
			return false
		}
	}
	return true
}

func replaceChain(bc []*Block) {
	if isValidChain(bc) && len(bc) > len(blockChain) {
		log.Println("Received blockchain is valid.Replacing current blockchain with received blockchain!")
		blockChain = bc
		broadcast(responseLatestMsg())
	} else {
		log.Println("Received blockchain invalid!")
	}
}

func queryAllMsg() []byte {
	return []byte(fmt.Sprintf("{\"type\":%d}", queryAll))
}

func broadcast(msg []byte) {
	for n, socket := range sockets {
		_, err := socket.Write(msg)
		if err != nil {
			log.Printf("peer[%s]disconnected", socket.RemoteAddr().String())
			sockets = append(sockets[0:n], sockets[n+1:]...)
		}
	}
}

func getLatestBlock() (b *Block) {
	return blockChain[len(blockChain)-1]
}

func responseLatestMsg() (bs []byte) { // ResponseLatestMsg response latest msg
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

func handleBlocks(w http.ResponseWriter, r *http.Request) {
	bs, err := json.Marshal(blockChain)
	if err != nil {
		return
	}
	i, err := w.Write(bs)
	if err != nil {
		return
	}
	fmt.Println("i = ", i)
}

func handleMineBlock(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Data string `json:"data,omitempty"`
	}
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&v)
	if err != nil {
		w.WriteHeader(http.StatusGone)
		log.Println("[API] invalid block data:", err.Error())
		w.Write([]byte("invalid block data." + err.Error() + "\n"))
		return
	}
}

func handlePeers(w http.ResponseWriter, r *http.Request) {
	var slice []string
	for _, socket := range sockets {
		if socket.IsClientConn() {
			slice = append(slice, strings.Replace(socket.LocalAddr().String(), "ws://", "", 1))
		} else {
			slice = append(slice, socket.Request().RemoteAddr)
		}
	}
	bs, _ := json.Marshal(slice)
	w.Write(bs)
}

func handleAddPeer(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Peer string `json:"peer,omitempty"`
	}
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(&v)
	if err != nil {
		w.WriteHeader(http.StatusGone)
		log.Println("[API] invalid peer data:", err.Error())
		w.Write([]byte("invalid peer data." + err.Error()))
		return
	}
	connectToPeers([]string{v.Peer})
}

func main() {
	flag.Parse()
	connectToPeers(strings.Split(*InitialPeers, ","))

	http.HandleFunc("/blocks", handleBlocks)
	http.HandleFunc("/mine_block", handleMineBlock)
	http.HandleFunc("/peers", handlePeers)
	http.HandleFunc("add_peer", handleAddPeer)

	go func() {
		log.Println("Listen HTTP on", *httpPort)
		errFatal("start api server", http.ListenAndServe(*httpPort, nil))
	}()
	http.Handle("/", websocket.Handler(wsHandleP2P))
	log.Println("Listen p2p on", *p2pPort)
	errFatal("start p2p server", http.ListenAndServe(*p2pPort, nil))
}
