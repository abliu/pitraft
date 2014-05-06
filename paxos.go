package main

import (
	"net"
	"log"
	"os"
	"time"
	"math/rand"
	"encoding/json"
	"flag"
	"strconv"
	"fmt"
)

// Important Paxos protocol constants.
const (
	ROUND_TRIP_TIME = 50 * time.Millisecond /*ms*/
	HEARTBEAT_TIME = 8 * ROUND_TRIP_TIME
	REQUEST_RETRY_TIME = 10 * ROUND_TRIP_TIME
	PRESIDENCY_TIME = 16 * ROUND_TRIP_TIME
	PORT_BASE = 23456
)

// multi-Paxos states
const (
	STOPPED = iota
	STARTING
	RUNNING
)

// Data about a peer taken from the command line parameters.
type Peer struct {
	address string
	index int
}

// Data about a peer that's internal to the Paxos protocol.
type PaxosPeer struct {
	info Peer
	// Parsed version of info.address
	resolvedAddr *net.UDPAddr
	// Time that we last received a heartbeat from this peer. For determining who's president.
	lastHeartbeat time.Time
}

// Data about the Paxos protocol. Most of this is internal. The
// interface that clients care about are the two channels 'requests'
// (for input) and 'decrees' (for output). The protocol works as
// follows:
//
// - A client puts a []byte in the decrees channel.
// - A Paxos thread picks it up and tries to send it to the president
// - It repeatedly re-tries until it sees the proposal come out the
// output side
// 
// Unfortunately, we did not have time to build a goraft-compatible
// server on top of this Paxos due to some coordination problems with
// the interface design. Hence all we have is a test case that shows
// achieving consensus on 10 sample values with n clients (simply run
// 'go run paxos.go <n>' to observe this).
type Paxos struct {
	// GENERAL DATA
	// A list of peers we talk do. Does not include ourself.
	peers []PaxosPeer
	// Info about ourself.
	self Peer
	// A channel of requests that the library client sends to us.
	requests chan []byte
	// The ledger.
	ledger [][]byte
	// An output channel for decrees.
	decrees chan []byte
	// Log for debug output.
	log *log.Logger
	// Set this to false to stop us from participating.
	running bool
	// Our communications channel with other nodes.
	conn *net.UDPConn
	quorumSize int

	// LEARNER DATA
	// A channel to store proposed values received from clients.
	proposals chan *PaxosValue
	// The last ledger item output on the channel.
	nextNil int
	// map seqnum -> accepted value
	acceptData map[int]PaxosValue

	// ACCEPTOR DATA
	// map containing IDs of accepted proposals
	acceptedProposals map[int64]bool
	// last seen and last prepared proposal numbers
	lastPrepared ProposalNumber
	lastAccepted ProposalNumber
	lastRecord PaxosRecord

	// PROPOSER DATA
	// the number of the current proposal we're using
	currentProposal ProposalNumber
	// the record we're currently trying to pass
	currentRecord PaxosRecord
	// the number of the highest previously accepted proposal
	otherLastAccepted ProposalNumber
	// the seqnum of ditto
	otherLastRecord PaxosRecord
	// which acceptors we've received a promise from so far
	receivedPromise map[int]bool
	// number of promises we've received
	numPromises int
	// which acceptors have accepted the current record
	acceptedRecord map[int]bool
	// how many acceptances
	numAccepted int
	// the quorum we're sending to (array of indices)
	quorum []int
	// whether we're currently a distinguished proposer
	multiPaxos int
}

// The type of a proposal number, so that we can order them properly
// with Compare(a,b) below.
type ProposalNumber struct {
	number int
	peerIndex int
}
func Compare(n1, n2 ProposalNumber) int {
	d := n1.number - n2.number
	if d == 0 {
		d = n1.peerIndex - n2.peerIndex
	}
	return d
}

// Initialize the Paxos object and start the protocol. We assume that
// we "magically" get a list of peers with unique indices from
// somewhere (aka a shared config).

// Parameters:
//   self: a description of the current peer (most importantly, an index)
//   peers: an array of peers that does NOT contain self.
func startPaxos(self Peer, peers []Peer) (p *Paxos, err error) {
	// log any errors
	defer (func() {
		if err != nil {
			p.log.Printf("ERR: %s\n", err.Error())
		}
	})()

	p = new(Paxos)
	p.self = self
	p.log = log.New(os.Stderr, fmt.Sprintf("paxos-%d: ", self.index), log.Lmicroseconds)
	p.log.Printf("starting Paxos node %d at %s\n", self.index, self.address)

	// start listening for connections
	address, err := net.ResolveUDPAddr("udp", self.address)
	if err != nil {
		return
	}
	p.conn, err = net.ListenUDP("udp", address)
	if err != nil {
		return
	}

	// parse my peers' addresses
	p.peers = make([]PaxosPeer, len(peers), len(peers))
	p.quorumSize = (len(peers)+1)/2
	for i, peer := range peers {
		log.Printf("dialing peer %d at %s", peer.index, peer.address)
		p.peers[i].info = peer
		p.peers[i].resolvedAddr, err = net.ResolveUDPAddr("udp", peer.address)
		if err != nil {
			return
		}
	}

	// set up the send/receive channels, ledger, and temporary storage
	// unfortunately Go seems not to have default struct initializers...
	p.requests = make(chan []byte, 1)
	p.ledger = make([][]byte, 100)
	p.decrees = make(chan []byte, 10)
	p.proposals = make(chan *PaxosValue, 100)
	p.acceptedProposals = make(map[int64]bool)
	// make sure we accept proposals with n=(0,0)
	p.lastAccepted.number = -1
	p.lastPrepared.number = -1
	p.currentProposal.number = -1
	p.currentRecord.Seqnum = -1
	p.otherLastAccepted.number = -1
	return
}

func (p *Paxos) Start() {
	// start running
	p.running = true

	for i := range p.peers {
		p.peers[i].lastHeartbeat = time.Now()
	}
	
	// Do the computing in three separate threads.

	// HandleRequests is responsible for reading request []bytes off of
	// the appropriate channel, figuring out who's president, and
	// passing on the requests to them.
	go p.HandleRequests()
	// This one is self-explanatory.
	go p.SendHeartbeat()
	// This thread loops through packets sent from other machines and
	// dispatches handlers. We try to keep handlers on the main thread
	// where possible, to avoid synchronization issues; the only place
	// we fork from here is that the president spins up another thread
	// to process the queue of decree requests (since we need to block
	// until things appear in that queue). See StartNextRecord() which
	// contains the blocking call.
	go p.HandleMessages()
}

////////////////////////////////////////////////////////////////////////////////
// PAXOS PRIMITIVES
////////////////////////////////////////////////////////////////////////////////

// Variant type for the Paxos wire protocol. Whichever command this
// is, that pointer gets set to non-nil; this is a principled and
// type-safe way of having polymorphic commands on the wire without
// doing crazy code-sharing declarations. The wire protocol is
// json-based because Go has excellent facilities for using json.
type PaxosMessage struct {
	SenderIndex int
	Heartbeat bool `json:",omitempty"`
	Prepare *Prepare `json:",omitempty"`
	Promise *Promise `json:",omitempty"`
	Accept *Accept `json:",omitempty"`
	Accepted *Accepted `json:",omitempty"`
	Request *PaxosValue `json:",omitempty"`
	Commit *Commit `json:",omitempty"`
}

// Create a message with the given type and data.
func (p *Paxos) NewPaxosMessage() *PaxosMessage {
	pm := new(PaxosMessage)
	pm.SenderIndex = p.self.index
	return pm
}

// Message type: request that the president issue a proposal.
type PaxosValue struct {
	Id int64
	Proposal []byte
}

// Requests get turned into Record objects by the president so they
// can carry a sequence number around with them while we're trying to
// pass them.
type PaxosRecord struct {
	Seqnum int
	Val PaxosValue
}

// Create a new proposal request object, returning the message and the
// request ID.
func (p *Paxos) NewProposalRequest(request []byte) (*PaxosMessage, int64) {
	m := p.NewPaxosMessage()
	m.Request = &PaxosValue{Proposal: request, Id: rand.Int63()}
	return m, m.Request.Id
}

// Message type: heartbeat
func (p *Paxos) NewHeartbeat() *PaxosMessage {
	m := p.NewPaxosMessage()
	m.Heartbeat = true
	return m
}

// Message type: prepare
type Prepare struct {
	// The number of the proposal, for priority
	N ProposalNumber
}
func (p *Paxos) NewPrepare(n ProposalNumber) *PaxosMessage {
	// pick a number higher than any we've seen
	m := p.NewPaxosMessage()
	m.Prepare = &Prepare{N: n}
	return m
}

// Message type: promise
type Promise struct {
	N ProposalNumber
	LastAccepted ProposalNumber
	LastRecord PaxosRecord
}
func (p *Paxos) NewPromise(pr *Prepare) *PaxosMessage {
	m := p.NewPaxosMessage()
	m.Promise = &Promise{
		N: pr.N,
		LastAccepted: p.lastAccepted,
		LastRecord: p.lastRecord,
	}
	return m
}

// Message type: accept request
type Accept struct {
	N ProposalNumber
	Record PaxosRecord
}
func (p *Paxos) NewAccept(pn ProposalNumber, r PaxosRecord) *PaxosMessage {
	m := p.NewPaxosMessage()
	m.Accept = &Accept{
		N: pn,
		Record: r,
	}
	return m
}

// Message type: accepted
type Accepted struct {
	N ProposalNumber
	Seqnum int
	Id int64
	// we don't need to include the value we accept; the id takes care of it
}
func (p *Paxos) NewAccepted(accept *Accept) *PaxosMessage {
	m := p.NewPaxosMessage()
	m.Accepted = &Accepted{
		N: accept.N,
		Seqnum: accept.Record.Seqnum,
		Id: accept.Record.Val.Id,
	}
	return m
}

type Commit struct {
	Record PaxosRecord
}

func (p *Paxos) NewCommit(r PaxosRecord) *PaxosMessage {
	m := p.NewPaxosMessage()
	m.Commit = &Commit{Record: r}
	return m
}

// Get the current president of the Paxos instance, based on the
// heartbeat data.
func (p *Paxos) GetPresident() int {
	pres := p.self.index
	for i := range p.peers {
		cur := &p.peers[i]
		if cur.info.index < pres && time.Since(cur.lastHeartbeat) < PRESIDENCY_TIME {
			pres = cur.info.index
		}
	}
	return pres
}
func (p *Paxos) IsPresident() bool {
	return p.GetPresident() == p.self.index
}

// Get a quorum of acceptors
func (p *Paxos) GetQuorum() []int {
	return nil
}

// Send the given message to the given peer.
func (p *Paxos) SendMessage (m *PaxosMessage, peer *PaxosPeer) (err error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		p.log.Println(err)
	}
	_, err = p.conn.WriteTo(bytes, peer.resolvedAddr)
	return
}

// Accept a decree and enter it into the ledger in the appropriate spot.
func (p *Paxos) AcceptDecree(i int, decree []byte) {
	// enlarge the ledger if necessary
	if i > cap(p.ledger) {
		newLedger := make([][]byte, cap(p.ledger)*2)
		copy(newLedger, p.ledger)
		p.ledger = newLedger
	}
	// make room for the new element (intermediate ones become nil)
	if len(p.ledger) < i {
		p.ledger = p.ledger[0:i+1]
	}
	p.ledger[i] = decree
	// If we've already output everything up to this decree, output the
	// new decree and everything available afterwards.
	for ; p.ledger[p.nextNil] != nil; p.nextNil += 1 {
		p.decrees <- p.ledger[p.nextNil]
	}
}

// Get the peer with the appropriate index. Does not handle the case
// that index == p.self.index, since we don't have a PaxosPeer for
// self and that should usually be handled separatedly anyway.
func (p *Paxos) GetPeer(index int) *PaxosPeer {
	for i := range p.peers {
		if p.peers[i].info.index == index {
			return &p.peers[i]
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PAXOS LOGIC
////////////////////////////////////////////////////////////////////////////////

// Read requests from the client channel and try to send them to the
// president. Here we play the role of client to the Paxos cluster.
func (p *Paxos) HandleRequests() {
	for p.running {
		request := <- p.requests
		go p.HandleRequest(request)
	}
}

func (p *Paxos) HandleRequest(request []byte) {
	p.log.Printf("received request: %v", request)
	pr, id := p.NewProposalRequest(request)
	// wait until we have a president, then continuously bug the
	// president until the request makes it into the ledger
	for p.running {
		pnum := p.GetPresident()
		// Are we the president? If so, handle the request directly rather
		// than going over UDP.
		if pnum == p.self.index {
			p.HandleProposalRequest(nil, pr.Request)
		} else {
			president := p.GetPeer(pnum)
			if president == nil {
				p.log.Println("waiting for president %d...", pnum)
				time.Sleep(ROUND_TRIP_TIME)
				continue
			}
			p.log.Printf("sending to president %d\n", president.info.index)
			p.SendMessage(pr, president)
		}
		// wait for a while and see if the proposal passed
		time.Sleep(REQUEST_RETRY_TIME)
		if p.acceptedProposals[id] {
			p.log.Printf("proposal %d passed!", id)
			return
		}
		p.log.Printf("proposal %d has not passed yet; retrying...", id)
	}
}

func (p *Paxos) SendHeartbeat() {
	p.log.Println("Starting heartbeat")
	interval := time.Duration(HEARTBEAT_TIME)
	heartbeat := p.NewHeartbeat()
	for p.running {
		time.Sleep(interval)
		for i := range p.peers {
			// send on a separate thread so we don't block for too long
			go p.SendMessage(heartbeat, &p.peers[i])
		}
	}
}

func (p *Paxos) HandleMessages() {
	p.log.Println("Waiting for messages")
	buf := make([]byte, 16384)
	for p.running {
		message := PaxosMessage{}
		n, err := p.conn.Read(buf)
		if err != nil {
			p.log.Println(err)
			continue
		}
		json.Unmarshal(buf[:n], &message)
		senderIndex := message.SenderIndex
		sender := p.GetPeer(senderIndex)
		switch {
		default:
			p.log.Println("Unknown type; ignoring")
		case message.Heartbeat:
			// There are n^2 of these messages, so let's not log
			//p.log.Println("Heartbeat")
			sender.lastHeartbeat = time.Now()
		case message.Request != nil:
			p.log.Println("Request")
			p.HandleProposalRequest(sender, message.Request)
		case message.Prepare != nil:
			p.log.Println("Prepare")
			p.HandlePrepare(sender, message.Prepare)
		case message.Promise != nil:
			p.HandlePromise(sender, message.Promise)
		case message.Accept != nil:
			p.HandleAccept(sender, message.Accept)
		case message.Accepted != nil:
			p.log.Println("Accepted")
			p.HandleAccepted(sender, message.Accepted)
		case message.Commit != nil:
			p.log.Println("Commit")
			p.HandleCommit(sender, message.Commit)
		}
	}
}

func (p *Paxos) HandleProposalRequest(sender *PaxosPeer, req *PaxosValue) {
	p.log.Printf("Request %d: %v", req.Id, req.Proposal)
	if p.IsPresident() {
		// we are the president; queue the proposal to be executed
		p.proposals <- req
		if p.multiPaxos == STOPPED {
			p.StartMultiPaxos()
		}
	} else {
		p.log.Println("I'm not the president!")
	}
}

func (p *Paxos) HandlePrepare(sender *PaxosPeer, prepare *Prepare) {
	p.log.Printf("Preparing request (%d, %d) from peer %d\n",
		prepare.N.number, prepare.N.peerIndex, sender.info.index)
	// first of all, if we were trying to pass a different proposal, we
	// need to stop that
	if Compare(p.currentProposal, prepare.N) < 0 {
		p.currentProposal.number = -1
		p.otherLastAccepted.number = -1
		p.receivedPromise = nil
		p.numPromises = 0
		// also stop our attempt to do multi-paxos, someone else is taking
		// over (we'll take it back if they fail because we'll end up
		// being president)
		p.multiPaxos = STOPPED
	}
	// Respond to this prepare request, if we can. Note that a
	// promise(n) doesn't preclude responding to other proposals with
	// number exactly n, which is important for multi-Paxos where the
	// proposer issues Prepare(n, i) for many values of i.
	cmp := Compare(p.lastPrepared, prepare.N)
	if cmp <= 0 {
		promise := p.NewPromise(prepare)
		if cmp < 0 {
			// the proposer changed! better throw out our state
			p.acceptData = make(map[int]PaxosValue)
		}
		p.lastPrepared = prepare.N
		p.log.Println("Replying with a promise")
		p.SendMessage(promise, sender)
	}
}

// Count the number of promises we collect, and move forwards once a
// quorum is reacherd.
func (p *Paxos) HandlePromise(sender *PaxosPeer, promise *Promise) {
	if promise.N == p.currentProposal && !p.receivedPromise[sender.info.index] {
		p.log.Printf("Received a correct promise from %d...", sender.info.index)
		p.receivedPromise[sender.info.index] = true
		p.numPromises += 1
		// figure out what the last thing we were trying to pass was
		if Compare(p.otherLastAccepted, promise.N) < 0 {
			p.otherLastAccepted = promise.LastAccepted
			p.otherLastRecord = promise.LastRecord
		}
		// comparing equality is ok because numPromises is only touched
		// from the main paxos thread
		if p.numPromises == p.quorumSize {
			p.log.Println("A quorum has been prepared. Executing Multi-Paxos.")
			go p.RunMultiPaxos()
		}
	}
}

// Try to become the distinguished proposer.
func (p *Paxos) StartMultiPaxos() {
	if p.multiPaxos != STOPPED {
		p.log.Println("already in multi-Paxos")
		return
	}
	p.log.Println("starting multi-Paxos...")
	p.multiPaxos = STARTING
	p.currentProposal.number = p.lastPrepared.number + 1
	p.currentProposal.peerIndex = p.self.index
	p.otherLastAccepted.number = -1
	p.receivedPromise = make(map[int]bool)
	// prepare all our peers so we can pick any quorum later
	m := p.NewPrepare(p.currentProposal)
	for i := range p.peers {
		p.SendMessage(m, &p.peers[i])
	}
}

// We've collected enough promises for a quorum. Now we can Start
// issuing the proposals that are in our queue.
func (p *Paxos) RunMultiPaxos() {
	if p.multiPaxos == RUNNING {
		p.log.Println("WARNING: already doing multi-Paxos")
		return
	}
	p.multiPaxos = RUNNING
	// first, if we were already trying to pass a thing, continue trying
	// to pass the thing
	if p.otherLastAccepted.number >= 0 && Compare(p.otherLastAccepted, p.currentProposal) < 0 {
		p.log.Printf("starting with the previous record from %v", p.otherLastAccepted)
		p.StartRecord(p.otherLastRecord)
	} else {
		// otherwise, start pulling from our internal proposals channel
		go p.StartNextRecord()
	}
}

func (p *Paxos) StartRecord(r PaxosRecord) {
	p.numAccepted = 0
	p.acceptedRecord = make(map[int]bool)
	p.currentRecord = r
	// Accept the thing ourselves
	p.AcceptRecord(p.currentProposal, r)
	m := p.NewAccept(p.currentProposal, r)
	// Send it out to all our guys
	for i := range p.peers {
		go p.SendMessage(m, &p.peers[i])
	}
}

func (p *Paxos) StartNextRecord() {
	p.log.Println("Starting next record")
	// we have to zero things now in case we get more (extraneous)
	// Accept messages back for previous records
	p.numAccepted = 0
	p.acceptedRecord = make(map[int]bool)
	oldSeqnum := p.currentRecord.Seqnum
	p.currentRecord.Seqnum = -1
	if p.multiPaxos == RUNNING {
		req := <-p.proposals
		p.log.Println("Issuing decree", req.Id)
		//TODO proper seqnum
		r := PaxosRecord{Seqnum:oldSeqnum+1, Val:*req}
		p.StartRecord(r)
	}
}

// The local part of accepting the record
func (p *Paxos) AcceptRecord(n ProposalNumber, r PaxosRecord) {
	p.lastAccepted = n
	p.lastRecord = r
}

// Handle a request to accept the record.
func (p *Paxos) HandleAccept(sender *PaxosPeer, accept *Accept) {
	cmp := Compare(accept.N, p.lastPrepared)
	if (cmp < 0) {
		p.log.Printf("Proposal is old; skipping")
		return
	}
	if (cmp > 0) {
		p.log.Printf("Wasn't asked to pre-accept this one")
		return
	}
	p.AcceptRecord(accept.N, accept.Record)
	p.SendMessage(p.NewAccepted(accept), sender)
}

// Handle a response that a peer has accepted the record.
func (p *Paxos) HandleAccepted(sender *PaxosPeer, accepted *Accepted) {
	index := sender.info.index
	p.log.Printf("Got acceptance of (%d, %d) #%d from %d",
		accepted.N.number, accepted.N.peerIndex, accepted.Seqnum, index)
	// Keep track of how many acceptances we have. If we have enough, we
	// can commit it to the log. After we commit to the log, if we're
	// still distinguished, we can pull off another proposal and
	// StartRecord it.
	if accepted.N == p.currentProposal && accepted.Seqnum == p.currentRecord.Seqnum {
		if !p.acceptedRecord[index] {
			p.acceptedRecord[index] = true
			p.numAccepted += 1
		}
		if p.numAccepted == p.quorumSize {
			// Accepted by a majority. Inform the learners!
			p.SendCommitRecord(p.currentRecord)
			go p.StartNextRecord()
		}
	}
}

// Send "commit" messages to learners (aka everyone). Also commit it
// to our own learner.
func (p *Paxos) SendCommitRecord(r PaxosRecord) {
	p.CommitRecord(r)
	m := p.NewCommit(r)
	for i := range p.peers {
		go p.SendMessage(m, &p.peers[i])
	}
}

// Handle receiving a commit message
func (p *Paxos) HandleCommit(sender *PaxosPeer, c *Commit) {
	p.CommitRecord(c.Record)
}

// The local side of committing.
func (p *Paxos) CommitRecord(r PaxosRecord) {
	p.AcceptDecree(r.Seqnum, r.Val.Proposal)
	p.acceptedProposals[r.Val.Id] = true
	p.log.Printf("Decree %d committed", r.Val.Id)
}

////////////////////////////////////////////////////////////////////////////////
// main
////////////////////////////////////////////////////////////////////////////////

// A test case; coordinate on the values 1...10 requested from random peers.
func DoClient(ps []*Paxos) {
	vals := make([]byte, 10)
	for i := range vals {
		j := byte(rand.Intn(len(ps)))
		ps[j].requests <- []byte {byte(i), j}
		time.Sleep(1 * time.Second)
	}
	// Print out everyone's ledger; they should look the same and have
	// the numbers 1...10 in the first byte and random numbers in the
	// second.
	//
	// Note: we're not actually guaranteed that all the listeners have
	// the same values at this point (the Commit packets could have been
	// dropped, e.g.). So this is actually a test of stronger conditions
	// than just Paxos. On a local machine, things are fast enough that
	// I've never observed it not working.
	for i := range vals {
		for _, p := range ps {
			fmt.Printf("%v", p.ledger[i])
		}
		fmt.Printf("\n")
	}
	os.Exit(0)
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		os.Exit(1)
	}
	// how many peers?
	numPeers, err := strconv.ParseInt(args[0], 0, 32)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Paxos with %d instances\n", numPeers)

	// peer i will listen on portbase+i
	portbase := PORT_BASE

	peers := make([]Peer, numPeers)
	for i := range peers {
		peers[i].address = fmt.Sprintf("127.0.0.1:%d", portbase + i)
		peers[i].index = i
	}

	insts := make([]*Paxos, numPeers)

	for i := range peers {
		self := peers[i]
		// make the list of peers for an instance by excising self
		others := make([]Peer, numPeers-1)
		copy(others[:i], peers[:i])
		copy(others[i:], peers[i+1:])
		inst, err := startPaxos(self, others)
		insts[i] = inst
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Created instance")
	}

	for _, p := range insts {
		p.Start()
	}

	// run the test
	go DoClient(insts)
	// the test takes care of exiting, so just sleep for a while
	time.Sleep(time.Second * 300)
}
