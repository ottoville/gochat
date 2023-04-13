package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"nhooyr.io/websocket"
)

// chatServer is the WebSocket server implementation.
// It ensures the client speaks the chat subprotocol and
// only allows one message every 100ms with a 10 message burst.
type chatServer struct {
	// logf controls where logs are sent.
	logf func(f string, v ...interface{})
}

// var nicks []string
var connections = make(map[*websocket.Conn]string)

func (s chatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nick := r.URL.Path[1:len(r.URL.Path)]
	log.Printf("new connection " + nick)
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:       []string{"chat"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logf("%v", err)
		return
	}
	connections[c] = nick
	ctx := context.Background()
	broadcastNicks(ctx)
	defer broadcastNicks(ctx)
	defer broadcast(ctx, 1, bytes.Join([][]byte{[]byte("{\"type\":\"message\", \"data\": \"" + nick + " has left the chat\"} ")}, nil))
	err3 := broadcast(ctx, 1, bytes.Join([][]byte{[]byte("{\"type\":\"message\", \"data\": \"" + nick + " joined\"} ")}, nil))
	if err3 != nil {
		s.logf("%v", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")
	defer delete(connections, c)
	defer log.Printf("Delete connection " + nick)

	if c.Subprotocol() != "chat" {
		log.Printf("error wrong subprotocol " + c.Subprotocol())

		c.Close(websocket.StatusPolicyViolation, "client must speak the chat subprotocol")
		return
	}

	l := rate.NewLimiter(rate.Every(time.Millisecond*100), 10)
	for {
		err = listen(r.Context(), c, l)
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return
		}
		if err != nil {
			s.logf("failed to chat with %v: %v", r.RemoteAddr, err)
			return
		}
	}
}
func broadcastNicks(ctx context.Context) error {
	nicks := make([]string, 0, len(connections))
	for _, value := range connections {
		nicks = append(nicks, value)
	}
	json, err := json.Marshal(nicks)
	if err != nil {
		return err
	}
	return broadcast(ctx, 1, bytes.Join([][]byte{[]byte("{\"type\":\"nicks\", \"data\":"), json, []byte("}")}, nil))

}
func broadcast(ctx context.Context, typ websocket.MessageType, bytes []byte) error {
	for conn := range connections {
		w2, err := conn.Writer(ctx, typ)
		if err != nil {
			return err
		}
		_, err = w2.Write(bytes)
		if err != nil {
			return fmt.Errorf("failed to io.Write: %w", err)
		}

		err = w2.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// listen reads from the WebSocket connection and then writes
// the received message back to it.
// The entire function has 6 hour to complete.
func listen(ctx context.Context, c *websocket.Conn, l *rate.Limiter) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60*60*6)
	defer cancel()

	err := l.WaitN(ctx, 1)
	if err != nil {
		return err
	}
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return err
	}
	messageBytes, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	jsonMessage, err := json.Marshal(string(messageBytes))
	if err != nil {
		return err
	}
	messageBytesJoined := bytes.Join([][]byte{[]byte("{\"type\":\"message\", \"data\":"), jsonMessage, []byte("}")}, nil)
	return broadcast(ctx, typ, messageBytesJoined)
}
