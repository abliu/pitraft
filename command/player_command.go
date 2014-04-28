package command

import (
	"github.com/abliu/pitraft"
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
    // TODO: Check for errors (e.g. player already exists)
	db := server.Context().(*db.DB)
    switch c.action {
        case "add":
            db.AddPlayer(c.playerId)
        case "remove":
            db.RemovePlayer(c.PlayerId)
        default:
            // DO SOMETHING
    }
	return nil, nil
}
