package player

const startingResources = 2

var resources = []string{"flax", "hay", "oats", "rye", "corn", "barley",
"wheat"}

//type Resource int

//const (
//    Flax = iota
//    Hay = iota
//    Oats = iota
//    Rye = iota
//    Corn = iota
//    Barley = iota
//    Wheat = iota
//)

type Player struct {
    //TODO: Un-export field
    Cards   map[string]int
    id      int
}

// Creates a new player.
func New(newId int) *Player {
    var defaultCards = make(map[string]int)
    for _, resource := range resources {
        defaultCards[resource] = startingResources
    }
    return &Player{
        Cards:  defaultCards,
        id:     newId,
    }
}
