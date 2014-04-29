package command

import (
    "github.com/goraft/raft"
    "github.com/abliu/pitraft/db"
)

// This command writes a value to a key.
type AddTradeCommand struct {
	Player      int     `json:"player"`
	Resource    string  `json:"resource"`
    Amount      int     `json:"amount"`
}

// Creates a new write command.
func NewAddTradeCommand(playerId int, resource string, amount int) *AddTradeCommand {
	return &AddTradeCommand{
		Player:     playerId,
        Resource:   resource,
        Amount:     amount,
	}
}

// The name of the command in the log.
func (c *AddTradeCommand) CommandName() string {
	return "addTrade"
}

// Writes a value to a key.
func (c *AddTradeCommand) Apply(server raft.Server) (interface{}, error) {
    // TODO: Check for errors (e.g. player already exists, other c.Action)
	pairdb := server.Context().(*db.PairDB)
    db := pairdb.TradeDB
    db.Add(c.Player, c.Resource, c.Amount)
	return nil, nil
}
