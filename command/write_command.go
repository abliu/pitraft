package command

import (
    "github.com/goraft/raft"
	"github.com/abliu/pitraft/db"
)

// This command writes a value to a key.
type WriteCommand struct {
	Player      int     `json:"player"`
	Resource    string  `json:"resource"`
    Amount      int     `json:"amount"`
}

// Creates a new write command.
func NewWriteCommand(playerId int, resource string, amount int) *WriteCommand {
	return &WriteCommand{
		Player:     playerId,
		Resource:   resource,
        Amount:     amount,
	}
}

// The name of the command in the log.
func (c *WriteCommand) CommandName() string {
	return "write"
}

// Writes a value to a key.
func (c *WriteCommand) Apply(server raft.Server) (interface{}, error) {
	db := server.Context().(*db.DB)
	db.Put(c.Player, c.Resource, c.Amount)
	return nil, nil
}
