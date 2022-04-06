package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn *websocket.Conn

	send chan []byte
}

var clients = make([]*Client, 0)

func socketHandler(c echo.Context) error {
	w := c.Response()
	r := c.Request()

	conn, err := upgrader.Upgrade(w, r, nil)
	close := true
	if err != nil {
		return nil
	}

	defer func() {
		if close {
			conn.Close()
		}
	}()

	// Try to read authentication key
	conn.SetReadDeadline(time.Now().Add(time.Second * 15))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil
	}

	message := string(msg)
	if len(message) == 133 && strings.HasPrefix(message, "auth ") {
		input_key := strings.TrimPrefix(message, "auth ")

		if input_key == secret_key {
			// Save client
			client := Client{conn: conn, send: make(chan []byte, 64)}
			clients = append(clients, &client)

			conn.WriteMessage(websocket.TextMessage, []byte("{\"type\": \"AUTH_OK\", \"data\": null}"))
			go client.writePump()
			go client.readPump()
			close = false

			return nil
		}
	}

	conn.WriteMessage(websocket.TextMessage, []byte("{\"type\": \"AUTH_ERROR\", \"data\": null}"))
	return nil
}

func SendSocketMessage(msgtype string, data interface{}) {
	json_string, err := json.Marshal(map[string]interface{}{"type": msgtype, "data": data})
	if err != nil {
		return
	}
	for _, client := range clients {
		client.send <- json_string
	}
}

func (c *Client) writePump() {
	defer func() {
		removeClient(c)
		c.conn.Close()
	}()

	for {
		message, ok := <-c.send
		if !ok {
			return
		}

		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("error writing socket message: %v", err)
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		removeClient(c)
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Time{}) // Reset read limit
	c.conn.SetReadLimit(1)
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func removeClient(c *Client) {
	for index, client := range clients {
		if client == c {
			clients[index] = clients[len(clients)-1]
			clients = clients[:len(clients)-1]
		}
	}
}
