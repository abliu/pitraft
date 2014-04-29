package command

import (
    "github.com/goraft/raft"
    "github.com/abliu/pitraft/db"
)

// This command writes a value to a key.
type RemoveTradeCommand struct {
	TradeId int `json:"tradeId"`
}

// Creates a new write command.
func NewRemoveTradeCommand(tradeId int) *RemoveTradeCommand {
	return &RemoveTradeCommand{
		TradeId:    tradeId,
	}
}

// The name of the command in the log.
func (c *RemoveTradeCommand) CommandName() string {
	return "removeTrade"
}

// Writes a value to a key.
func (c *RemoveTradeCommand) Apply(server raft.Server) (interface{}, error) {
    // TODO: Check for errors (e.g. player already exists, other c.Action)
	pairdb := server.Context().(*db.PairDB)
    db := pairdb.TradeDB
    db.Remove(c.TradeId)
	return nil, nil
}
