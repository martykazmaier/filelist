package comm

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Stream interface {
	io.ReadWriteCloser
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type rawStream struct {
	c net.Conn
}

func (r *rawStream) Read(p []byte) (int, error)  { return r.c.Read(p) }
func (r *rawStream) Write(p []byte) (int, error) { return r.c.Write(p) }
func (r *rawStream) Close() error                { return r.c.Close() }
func (r *rawStream) SetReadDeadline(t time.Time) error {
	return r.c.SetReadDeadline(t)
}
func (r *rawStream) SetWriteDeadline(t time.Time) error {
	return r.c.SetWriteDeadline(t)
}

func WrapRaw(c net.Conn) Stream {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
		_ = tc.SetKeepAlive(true)
	}
	return &rawStream{c: c}
}

func Wrap(c net.Conn, websocketMode bool) Stream {
	raw := WrapRaw(c)
	if websocketMode {
		return NewWS(c)
	}
	return raw
}

type wsStream struct {
	c      net.Conn
	mu     sync.Mutex
	rbuf   []byte
	closed bool
}

func NewWS(c net.Conn) Stream {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	return &wsStream{c: c}
}

func (w *wsStream) Read(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, io.EOF
	}
	for len(w.rbuf) == 0 {
		opcode, payload, err := readFrame(w.c)
		if err != nil {
			return 0, err
		}
		switch opcode {
		case websocket.TextMessage, websocket.BinaryMessage, 0:
			w.rbuf = payload
		case websocket.CloseMessage:
			w.closed = true
			return 0, io.EOF
		case websocket.PingMessage:
			_ = writeFrame(w.c, websocket.PongMessage, payload)
		case websocket.PongMessage:
		default:
		}
	}
	n := copy(p, w.rbuf)
	w.rbuf = w.rbuf[n:]
	return n, nil
}

func (w *wsStream) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	if err := writeFrame(w.c, websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsStream) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = writeFrame(w.c, websocket.CloseMessage, msg)
	return w.c.Close()
}

func (w *wsStream) SetReadDeadline(t time.Time) error {
	return w.c.SetReadDeadline(t)
}

func (w *wsStream) SetWriteDeadline(t time.Time) error {
	return w.c.SetWriteDeadline(t)
}

func writeFrame(c net.Conn, opcode int, payload []byte) error {
	hdr := make([]byte, 0, 10)
	hdr = append(hdr, byte(0x80|opcode))
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n < 65536:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127,
			0, 0, 0, 0,
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	if _, err := c.Write(hdr); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	_, err := c.Write(payload)
	return err
}

func readFrame(c net.Conn) (int, []byte, error) {
	var h [2]byte
	if err := readFull(c, h[:]); err != nil {
		return 0, nil, err
	}
	opcode := int(h[0] & 0x0F)
	masked := h[1]&0x80 != 0
	n := int(h[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if err := readFull(c, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int(ext[0])<<8 | int(ext[1])
	case 127:
		var ext [8]byte
		if err := readFull(c, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int(ext[4])<<24 | int(ext[5])<<16 | int(ext[6])<<8 | int(ext[7])
	}
	var mask [4]byte
	if masked {
		if err := readFull(c, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, n)
	if n > 0 {
		if err := readFull(c, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func readFull(r io.Reader, p []byte) error {
	_, err := io.ReadFull(r, p)
	return err
}
