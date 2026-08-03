package logstream

import (
	"github.com/muraenateam/muraena/log"
)

type Client struct {
	send chan log.LogLine
}

func NewClient() *Client                   { return &Client{send: make(chan log.LogLine, 64)} }
func (c *Client) Send() <-chan log.LogLine { return c.send }

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan log.LogLine
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan log.LogLine, 256),
	}
}

func (h *Hub) Register(c *Client)      { h.register <- c }
func (h *Hub) Unregister(c *Client)    { h.unregister <- c }
func (h *Hub) Broadcast(l log.LogLine) { h.broadcast <- l }

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if h.clients[c] {
				delete(h.clients, c)
				close(c.send)
			}
		case l := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- l:
				default: // slow client: drop this line for it
				}
			}
		}
	}
}
