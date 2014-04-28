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
    //cards map[Resource]int
    cards   map[string]int
    id      int
}

// Creates a new player.
func New(newId int) *Player {
    var defaultCards = map[string]int
    for resource := range resources {
        defaultCards[resource] = startingResources
    }
    return &Player{
        cards:  defaultCards,
        id:     newId
    }
}
