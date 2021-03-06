package main

import (
  "flag"
  "fmt"
  "os"
  "net/http"
  "strings"
  "encoding/json"
  "math/rand"
)

var wrUrl string
var rdUrl string

func init() {
  flag.StringVar(&wrUrl, "u", "127.0.0.1:4001", "leaderUrl")
  flag.Usage = func() {
    fmt.Fprintf(os.Stderr, "Usage: %s [arguments] \n", os.Args[0])
    flag.PrintDefaults()
  }
}

func main() {
  flag.Parse()

  uuid := rand.Int()
  uuidStr := fmt.Sprintf("%d", uuid)

  wrUrl = fmt.Sprintf("http://%s", wrUrl)
  tmp := []string{wrUrl, "addPlayer", uuidStr}
  resp, err := http.Get(strings.Join(tmp, "/"))
  fmt.Printf("%s::%v::%v\n\n", strings.Join(tmp, ""), *resp, err)

  var jsonResp map[string]int
  fmt.Printf("%v::%v\n", json.NewDecoder((*resp).Body).Decode(&jsonResp), jsonResp)

  for {
    fmt.Printf("> ")
    var cmd string
    fmt.Scanf("%s", &cmd)
    switch cmd {
      case "help":
        fmt.Printf("propose [resource] [amount]: propose a trade of [amount] of [resource]; returns tradeID and stats on trade proposed\ncancel [tradeID]: cancel a trade with ID [tradeID]\ngetState: get (my own) state\nview: view all live trades\n")
      case "propose":
        // propose trade (resource, amt)
        var resource string
        var amount string
        fmt.Printf("resource amt: ")
        fmt.Scanf("%s %s", &resource, &amount)
        fmt.Printf("\n")
        tmp = []string{wrUrl, "propTrade", uuidStr, resource, amount}
        resp, err = http.Get(strings.Join(tmp, "/"))
        if err != nil {
          fmt.Printf("Bad command %s %s %s::%s\n", cmd, resource, amount, err)
          continue
        }
        //var tradeID int
        b := (*resp).Body
        p := make([]byte, 1024, 1024)
        _, err := b.Read(p)
        if err != nil {
          fmt.Printf("Bad command %s %s %s::%s\n", cmd, resource, amount, err)
          continue
        }
        fmt.Printf("tradeID::%s::(%s, %s)\n", p, resource, amount)
      case "cancel":
        // cancel trade with given ID
        var tradeID string
        fmt.Printf("tradeID: ")
        fmt.Scanf("%s", &tradeID)
        fmt.Printf("\n")
        tmp = []string{wrUrl, "cancelTrade", tradeID}
        resp, err = http.Get(strings.Join(tmp, "/"))
        if err != nil {
          fmt.Printf("Bad command %s %s::%s\n", cmd, tradeID, err)
          continue
        }
      case "getState":
        // view my state
        tmp = []string{wrUrl, "gameState", uuidStr}
        resp, err = http.Get(strings.Join(tmp, "/"))
        if err != nil {
          fmt.Printf("Bad command::%s\n", err)
          continue
        } else {
          json.NewDecoder((*resp).Body).Decode(&jsonResp)
          fmt.Printf("%v\n", jsonResp)
        }
      case "view":
        // view trades
        tmp = []string{wrUrl, "allTrades"}
        resp, err = http.Get(strings.Join(tmp, "/"))
        if err != nil {
          fmt.Printf("Bad command::%s\n", err)
          continue
        } else {
          b := (*resp).Body
          p := make([]byte, 1024, 1024)
          _, err := b.Read(p)
          if err != nil {
            fmt.Printf("Error receiving response::%s\n", err)
            continue
          }
          fmt.Printf("%s\n", p)
        }
    }
  }
}
