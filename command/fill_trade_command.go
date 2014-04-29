package command

import (
    "github.com/goraft/raft"
    "github.com/abliu/pitraft/db"
)

type FillTradeCommand struct {
	TradeId         int     `json:"tradeId"`
    AskPlayer       int     `json:"askPlayer"`
    AskResource     string  `json:"askResource"`
    FillPlayer      int     `json:"fillPlayer"`
    FillResource    string  `json:"fillResource"`
    Amount          int     `json:"amount"`
}

// Creates a new write command.
func NewFillTradeCommand(tradeId int, askPlayer int, askResource string,
    fillPlayer int, fillResource string, amount int) *FillTradeCommand {
	return &FillTradeCommand{
		TradeId:        tradeId,
        AskPlayer:      askPlayer,
        AskResource:    askResource,
        FillPlayer:     fillPlayer,
        FillResource:   fillResource,
        Amount:         amount,
	}
}

// The name of the command in the log.
func (c *FillTradeCommand) CommandName() string {
	return "fillTrade"
}

// Writes a value to a key.
func (c *FillTradeCommand) Apply(server raft.Server) (interface{}, error) {
    // TODO: Check for errors (e.g. player already exists, other c.Action)
	pairdb := server.Context().(*db.PairDB)
    db := pairdb.DB
    tradedb := pairdb.TradeDB
    askHigh := db.GetPlayerResource(c.AskPlayer, c.AskResource)
    db.Put(c.AskPlayer, c.AskResource, askHigh - c.Amount)
    askLow := db.GetPlayerResource(c.AskPlayer, c.FillResource)
    db.Put(c.AskPlayer, c.FillResource, askLow + c.Amount)
    fillHigh := db.GetPlayerResource(c.FillPlayer, c.FillResource)
    db.Put(c.FillPlayer, c.FillResource, fillHigh - c.Amount)
    fillLow := db.GetPlayerResource(c.FillPlayer, c.AskResource)
    db.Put(c.FillPlayer, c.AskResource, fillLow + c.Amount)
    tradedb.Remove(c.TradeId)
	return nil, nil
}
