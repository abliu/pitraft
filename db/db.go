// Adapted from raftd: https://github.com/goraft/raftd.

package db

import (
	"sync"
    "github.com/abliu/pitraft/player"
    "github.com/abliu/pitraft/tradedb"
)

// The key-value database.
type DB struct {
	gameState  map[int](*player.Player) // maps player ids to players
	mutex      sync.RWMutex
}

// Creates a new database.
func New() *DB {
	return &DB{
		gameState:  make(map[int](*player.Player)),
	}
}

// Retrieves the value for a given key.
func (db *DB) Get() map[int](*player.Player) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.gameState
}

func (db *DB) GetPlayer(playerId int) map[string]int {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.gameState[playerId].Cards
}

func (db *DB) GetPlayerResource(playerId int, resource string) int {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.gameState[playerId].Cards[resource]
}

// Sets the value for a given key.
func (db *DB) Put(id int, resource string, amount int) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
    //TODO: check errors here
    db.gameState[id].Cards[resource] = amount
}

func (db *DB) AddPlayer(id int) {
    //TODO: check if player already exists
    db.mutex.Lock()
    defer db.mutex.Unlock()
    db.gameState[id] = player.New(id)
}

func (db *DB) RemovePlayer(id int) {
    //TODO: check if player does not exist
    db.mutex.Lock()
    defer db.mutex.Unlock()
    delete(db.gameState, id)
}

type PairDB struct {
    DB      *DB
    TradeDB *tradedb.TradeDB
}

