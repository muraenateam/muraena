package traffic

import (
	"github.com/muraenateam/muraena/core/capture"
)

type Summary struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	VictimID  string `json:"victimID"`
	Method    string `json:"method"`
	Host      string `json:"host"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	ReqSize   int    `json:"reqSize"`
	ResSize   int    `json:"resSize"`
}

func SummaryOf(f capture.Flow) Summary {
	return Summary{
		ID: f.ID, Timestamp: f.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		VictimID: f.VictimID, Method: f.Method, Host: f.Host, Path: f.Path,
		Status: f.Status, ReqSize: f.ReqSize, ResSize: f.ResSize,
	}
}

type Filter struct {
	Host     string
	VictimID string
	Method   string
	Status   int
}

func (f Filter) Match(s Summary) bool {
	if f.Host != "" && f.Host != s.Host {
		return false
	}
	if f.VictimID != "" && f.VictimID != s.VictimID {
		return false
	}
	if f.Method != "" && f.Method != s.Method {
		return false
	}
	if f.Status != 0 && f.Status != s.Status {
		return false
	}
	return true
}

type Client struct {
	Filter Filter
	send   chan Summary
}

func NewClient() *Client               { return &Client{send: make(chan Summary, 64)} }
func (c *Client) Send() <-chan Summary { return c.send }

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Summary
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Summary, 256),
	}
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }
func (h *Hub) Broadcast(s Summary)  { h.broadcast <- s }

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
		case s := <-h.broadcast:
			for c := range h.clients {
				if !c.Filter.Match(s) {
					continue
				}
				select {
				case c.send <- s:
				default: // slow client: drop this summary for it
				}
			}
		}
	}
}
