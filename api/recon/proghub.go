package recon

// ProgClient is a single WebSocket subscriber receiving recon progress strings.
type ProgClient struct{ send chan string }

func (c *ProgClient) Send() <-chan string { return c.send }

// ProgHub fans recon progress strings out to all registered clients. It mirrors
// the traffic hub but broadcasts plain strings instead of flow summaries.
type ProgHub struct {
	clients    map[*ProgClient]bool
	register   chan *ProgClient
	unregister chan *ProgClient
	broadcast  chan string
}

func NewProgHub() *ProgHub {
	return &ProgHub{
		clients:    map[*ProgClient]bool{},
		register:   make(chan *ProgClient),
		unregister: make(chan *ProgClient),
		broadcast:  make(chan string, 256),
	}
}

func (h *ProgHub) NewClient() *ProgClient   { return &ProgClient{send: make(chan string, 64)} }
func (h *ProgHub) Register(c *ProgClient)   { h.register <- c }
func (h *ProgHub) Unregister(c *ProgClient) { h.unregister <- c }
func (h *ProgHub) Broadcast(m string)       { h.broadcast <- m }

func (h *ProgHub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
		case c := <-h.unregister:
			if h.clients[c] {
				delete(h.clients, c)
				close(c.send)
			}
		case m := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- m:
				default:
				}
			}
		}
	}
}
