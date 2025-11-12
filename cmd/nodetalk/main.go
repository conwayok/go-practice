package main

import "fmt"

type Node struct {
	ID    int
	inbox chan Request
	peer  Peer
}

type Request struct {
	From    int
	Data    string
	ReplyCh chan Response
}
type Response struct {
	From int
	Data string
}

type Peer interface {
	Send(request Request)
}

type DirectPeer struct {
	node *Node
}

func (d *DirectPeer) Send(request Request) {
	d.node.inbox <- request
}

func (n *Node) StartConversation() {
	fmt.Println("Conversation started")

	replyCh := make(chan Response, 1)
	n.peer.Send(Request{From: n.ID, Data: "Hello", ReplyCh: replyCh})

	response := <-replyCh
	fmt.Printf("[node %d] response from node %d: %s\n", n.ID, response.From, response.Data)

	if response.Data != "Hey" {
		panic("unexpected response: " + response.Data)
	}

	replyCh = make(chan Response, 1)
	n.peer.Send(Request{From: n.ID, Data: "How are you", ReplyCh: replyCh})

	response = <-replyCh
	fmt.Printf("[node %d] response from node %d: %s\n", n.ID, response.From, response.Data)

	if response.Data != "I'm fine, thank you" {
		panic("unexpected response: " + response.Data)
	}

	fmt.Println("Conversation finished")
}

func (n *Node) Run() {
	for request := range n.inbox {
		fmt.Printf("[node %d] request from from node %d: %s\n", n.ID, request.From, request.Data)
		if request.Data == "Hello" {
			request.ReplyCh <- Response{From: n.ID, Data: "Hey"}
		} else if request.Data == "Hey" {
			request.ReplyCh <- Response{From: n.ID, Data: "How are you"}
		} else if request.Data == "How are you" {
			request.ReplyCh <- Response{From: n.ID, Data: "I'm fine, thank you"}
		} else {
			request.ReplyCh <- Response{From: n.ID, Data: "I don't understand"}
		}
		close(request.ReplyCh)
	}
}

func main() {
	// two "nodes" should talk to each other
	// the conversation should be:
	// node 1 -> node 2: "Hello"
	// node 2 -> node 1: "Hey"
	// node 1 -> node 2: "How are you"
	// node 2 -> node 1: "I'm fine, thank you"

	node1 := Node{ID: 1, inbox: make(chan Request)}
	node2 := Node{ID: 2, inbox: make(chan Request)}

	node1.peer = &DirectPeer{&node2}
	node2.peer = &DirectPeer{&node1}

	go node1.Run()
	go node2.Run()

	node1.StartConversation()

	close(node1.inbox)
	close(node2.inbox)
}
