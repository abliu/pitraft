package command

import (
    "github.com/goraft/raft"
	"github.com/abliu/pitraft/db"
)

// This command writes a value to a key.
type PlayerCommand struct {
	Player      int     `json:"player"`
	Action      string  `json:"action"`
}

// Creates a new write command.
func NewPlayerCommand(playerId int, action string) *PlayerCommand {
	return &PlayerCommand{
		Player: playerId,
		Action: action,
	}
}

// The name of the command in the log.
func (c *PlayerCommand) CommandName() string {
	return "player"
}

// Writes a value to a key.
func (c *PlayerCommand) Apply(server raft.Server) (interface{}, error) {
    // TODO: Check for errors (e.g. player already exists, other c.Action)
	pairdb := server.Context().(*db.PairDB)
    db := pairdb.DB
    switch c.Action {
        case "add":
            db.AddPlayer(c.Player)
        case "remove":
            db.RemovePlayer(c.Player)
    }
	return nil, nil
}
