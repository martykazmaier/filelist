package comm

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
	Subprotocols:    []string{"binary", "telnet"},
}

type gorillaNet struct {
	ws *websocket.Conn
	mu sync.Mutex
	r  []byte
}

func GorillaStream(ws *websocket.Conn) Stream {
	ws.SetReadLimit(64 * 1024)
	return &gorillaNet{ws: ws}
}

func UpgradeHTTP(w http.ResponseWriter, r *http.Request) (Stream, error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return GorillaStream(ws), nil
}

func (g *gorillaNet) Read(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for len(g.r) == 0 {
		_, data, err := g.ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		g.r = data
	}
	n := copy(p, g.r)
	g.r = g.r[n:]
	return n, nil
}

func (g *gorillaNet) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (g *gorillaNet) Close() error { return g.ws.Close() }

func (g *gorillaNet) SetReadDeadline(t time.Time) error {
	return g.ws.SetReadDeadline(t)
}

func (g *gorillaNet) SetWriteDeadline(t time.Time) error {
	return g.ws.SetWriteDeadline(t)
}
