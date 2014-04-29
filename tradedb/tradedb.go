package tradedb

import (
	"sync"
)

type Trade struct {
    Player      int
    Resource    string
    Amount      int
}

type TradeDB struct {
	trades  map[int](*Trade) 
    counter int
	mutex   sync.RWMutex
}

// Creates a new database.
func New() *TradeDB {
	return &TradeDB{
		trades:  make(map[int](*Trade)),
        counter: 0,
	}
}

// Retrieves all the trades.
func (db *TradeDB) Get() map[int](*Trade) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.trades
}

// Retrieves all the trades.
func (db *TradeDB) GetTrade(id int) *Trade {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.trades[id]
}

// Adds a trade.
func (db *TradeDB) Add(player int, resource string, amount int) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
    //TODO: check errors here
    db.counter++
    db.trades[db.counter] = &Trade{
        Player:     player,
        Resource:   resource,
        Amount:     amount,
    }
}

// Removes a trade.
func (db *TradeDB) Remove(id int) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
    //TODO: check errors here
    delete(db.trades, id)
}
