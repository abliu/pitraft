package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/goraft/raft"
	"github.com/abliu/pitraft/command"
	"github.com/abliu/pitraft/db"
	"github.com/abliu/pitraft/tradedb"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
    "strconv"
	"sync"
	"time"
)

// The pitraft server is a combination of the Raft server and an HTTP
// server which acts as the transport.
type Server struct {
	name       string
	host       string
	port       int
	path       string
	router     *mux.Router
	raftServer raft.Server
	httpServer *http.Server
	db         *db.DB
    tradedb    *tradedb.TradeDB
    pairdb     *db.PairDB
	mutex      sync.RWMutex
}

// Creates a new server.
func New(path string, host string, port int) *Server {
    adb := db.New()
    atradedb := tradedb.New()
	s := &Server{
		host:       host,
		port:       port,
		path:       path,
		db:         adb,
        tradedb:    atradedb,
		router:     mux.NewRouter(),
        pairdb:     &db.PairDB{
            DB:         adb,
            TradeDB:    atradedb,
        },
	}

	// Read existing name or generate a new one.
	if b, err := ioutil.ReadFile(filepath.Join(path, "name")); err == nil {
		s.name = string(b)
	} else {
		s.name = fmt.Sprintf("%07x", rand.Int())[0:7]
		if err = ioutil.WriteFile(filepath.Join(path, "name"), []byte(s.name), 0644); err != nil {
			panic(err)
		}
	}

	return s
}

// Returns the connection string.
func (s *Server) connectionString() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

// Starts the server.
func (s *Server) ListenAndServe(leader string) error {
	var err error

	log.Printf("Initializing Raft Server: %s", s.path)

	// Initialize and start Raft server.
	transporter := raft.NewHTTPTransporter("/raft", 200*time.Millisecond)
	s.raftServer, err = raft.NewServer(s.name, s.path, transporter, nil, s.pairdb, "")
	if err != nil {
		log.Fatal(err)
	}
	transporter.Install(s.raftServer, s)
	s.raftServer.Start()

	if leader != "" {
		// Join to leader if specified.

		log.Println("Attempting to join leader:", leader)

		if !s.raftServer.IsLogEmpty() {
			log.Fatal("Cannot join with an existing log")
		}
		if err := s.Join(leader); err != nil {
			log.Fatal(err)
		}

	} else if s.raftServer.IsLogEmpty() {
		// Initialize the server by joining itself.

		log.Println("Initializing new cluster")

		_, err := s.raftServer.Do(&raft.DefaultJoinCommand{
			Name:             s.raftServer.Name(),
			ConnectionString: s.connectionString(),
		})
		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("Recovered from log")
	}

	log.Println("Initializing HTTP server")

	// Initialize and start HTTP server.
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

    //TODO: differentiate get and post?
    s.router.HandleFunc("/addPlayer/{playerId}", s.addPlayerHandler).Methods("GET")
    s.router.HandleFunc("/removePlayer/{playerId}", s.removePlayerHandler).Methods("GET")
	s.router.HandleFunc("/gameState", s.readHandler).Methods("GET")
    s.router.HandleFunc("/gameState/{playerId}", s.readPlayerHandler).Methods("GET")

	s.router.HandleFunc("/join", s.joinHandler).Methods("POST")

    s.router.HandleFunc("/allTrades", s.readAllTradesHandler).Methods("GET")
    s.router.HandleFunc("/propTrade/{playerId}/{resource}/{amount}",
        s.propTradeHandler).Methods("GET")
    s.router.HandleFunc("/cancelTrade/{tradeId}",
        s.cancelTradeHandler).Methods("GET")
    s.router.HandleFunc("/acceptTrade/{tradeId}/{playerId}/{resource}",
        s.acceptTradeHandler).Methods("GET")

	log.Println("Listening at:", s.connectionString())

	return s.httpServer.ListenAndServe()
}

// This is a hack around Gorilla mux not providing the correct net/http
// HandleFunc() interface.
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.HandleFunc(pattern, handler)
}

// Joins to the leader of an existing cluster.
func (s *Server) Join(leader string) error {
	command := &raft.DefaultJoinCommand{
		Name:             s.raftServer.Name(),
		ConnectionString: s.connectionString(),
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(command)
	resp, err := http.Post(fmt.Sprintf("http://%s/join", leader), "application/json", &b)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func (s *Server) joinHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.DefaultJoinCommand{}

	if err := json.NewDecoder(req.Body).Decode(&command); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.raftServer.Do(command); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) readPlayerHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
    pid, err := strconv.ParseInt(vars["playerId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	gameState := s.db.Get()
    gameStateForPlayer := gameState[int(pid)].Cards
    msg, err := json.Marshal(gameStateForPlayer)
    w.Write([]byte(msg))
}

func (s *Server) readHandler(w http.ResponseWriter, req *http.Request) {
	//vars := mux.Vars(req)
	gameState := s.db.Get()
    gameStateStr := "Game state: "
    for _, pcards := range gameState {
        gameStateStr += fmt.Sprintf("%v;", pcards)
    }
    fmt.Fprintf(w, gameStateStr)
}

func (s *Server) readAllTradesHandler(w http.ResponseWriter, req *http.Request) {
	//vars := mux.Vars(req)
	trades := s.tradedb.Get()
    tradesStr := make(map[string](*tradedb.Trade))
    for id, trade := range(trades) {
        tradesStr[fmt.Sprintf("%d", id)] = trade
    }
    msg, err := json.Marshal(tradesStr)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
	}
    w.Write([]byte(msg))
}

func (s *Server) cancelTradeHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	// Execute the command against the Raft server.
    tradeId, err := strconv.ParseInt(vars["tradeId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    _, err = s.raftServer.Do(command.NewRemoveTradeCommand(int(tradeId)))
    if err != nil {
       http.Error(w, err.Error(), http.StatusBadRequest)
    }
}

func (s *Server) acceptTradeHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	// Execute the command against the Raft server.
    tradeId, err := strconv.ParseInt(vars["tradeId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    pid, err := strconv.ParseInt(vars["playerId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    // Check if both players have enough of each resource
    //TODO: Fix this check (notify players if not enough for trade)
    trade := s.tradedb.GetTrade(int(tradeId))
    askHand := s.db.GetPlayer(trade.Player)
    fillHand := s.db.GetPlayer(int(pid))
    amt := trade.Amount
    if askHand[trade.Resource] >= amt && fillHand[vars["resource"]] >= amt {
        _, err = s.raftServer.Do(command.NewFillTradeCommand(int(tradeId),
            trade.Player, trade.Resource, int(pid), vars["resource"], amt))
        if err != nil {
           http.Error(w, err.Error(), http.StatusBadRequest)
        }
    }
}

func (s *Server) propTradeHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	// Execute the command against the Raft server.
    pid, err := strconv.ParseInt(vars["playerId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    amount, err := strconv.ParseInt(vars["amount"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    // Check if player has appropriate resources
    //TODO: Better error handling here if not enough resources
    tradeId := rand.Int()
    if s.db.GetPlayer(int(pid))[vars["resource"]] >= int(amount) {
        _, err = s.raftServer.Do(command.NewAddTradeCommand(int(pid),
            vars["resource"], int(amount), tradeId))
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
        }
    }

    msg, _ := json.Marshal(tradeId)
    w.Write([]byte(msg))
}

func (s *Server) addPlayerHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

    //TODOs: check if player already exists, etc.
    pid, err := strconv.ParseInt(vars["playerId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    _, err = s.raftServer.Do(command.NewPlayerCommand(int(pid), "add"))
    if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (s *Server) removePlayerHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

    //TODOs: check if player already exists, etc.
    pid, err := strconv.ParseInt(vars["playerId"], 10, 0)
    if err != nil { //TODO: make more informative error
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
    _, err = s.raftServer.Do(command.NewPlayerCommand(int(pid), "remove"))
    if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
