package main

import (
  "flag"
  "fmt"
//  "github.com/goraft/raft"
//  "github.com/abliu/pitraft/pitServer"
//  "github.com/abliu/pitraft/command"
//  "github.com/abliu/pitraft/server"
  "os"
//  "io"
//  "net"
  "net/http"
//  "log"
//  "encoding/gob"
  "strings"
  "encoding/json"
)

/*var host string
var port int*/
var wrUrl string

func init() {
  /*flag.StringVar(&host, "h", "localhost", "hostname")
  flag.IntVar(&port, "p", 4002, "port")*/
  flag.StringVar(&wrUrl, "u", "http://127.0.0.1:4002/", "leaderUrl")
  flag.Usage = func() {
    fmt.Fprintf(os.Stderr, "Usage: %s [arguments] \n", os.Args[0])
    flag.PrintDefaults()
  }
}



func main() {
  flag.Parse()

  /*wrConn, err := net.Dial("tcp", wrUrl)
  if err != nil {
    log.Fatal("Could not connect to leader", err)
  }*/

  tmp := []string{wrUrl, "addPlayer", "/234"}
  resp, err := http.Get(strings.Join(tmp, ""))
  fmt.Printf("%s::%v::%v\n\n", strings.Join(tmp, ""), *resp, err)

  tmp = []string{wrUrl, "gameState"}
  resp, err = http.Get(strings.Join(tmp, ""))
  fmt.Printf("%s::%#v::%#v\n\n", strings.Join(tmp, ""), *resp, err)

  fmt.Printf("%v\n\n", (*resp).Body)
  b := (*resp).Body
  p := make([]byte, 1024, 1024)
  n, err := b.Read(p)
  fmt.Printf("%d::%s::%v\n\n", n, p, err)

/*
  encoder := gob.NewEncoder(wrConn)
  p := command.NewWriteCommand(1, "barley", 2)
  encoder.Encode(p)
  wrConn.Close()
*/
  fmt.Println("done")
}
