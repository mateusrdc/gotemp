package http

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"nhooyr.io/websocket"
)

var clients = make([]*websocket.Conn, 0)

func socketHandler(c echo.Context) error {
	conn, err := websocket.Accept(c.Response(), c.Request(), nil)
	if err != nil {
		return errors.New("couldn't establish websocket connection")
	}
	defer conn.Close(websocket.StatusGoingAway, "going away")
	defer removeClient(conn)

	conn.SetReadLimit(256)

	// Wait for messages
	loggedIn := false

	for {
		// Set a read deadline
		var ctx context.Context
		var cancel context.CancelFunc

		if !loggedIn {
			ctx, cancel = context.WithTimeout(context.Background(), time.Second*15)
		} else {
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute*2)
		}
		defer cancel()

		// Read message
		messageType, message, err := conn.Read(ctx)
		if err != nil {
			break
		}
		if messageType != websocket.MessageText {
			continue
		}

		parts := strings.SplitN(string(message), " ", 2)

		switch parts[0] {
		case "auth":
			if len(parts) < 2 {
				conn.Write(context.Background(), websocket.MessageText, []byte(`{"type": "AUTH_ERROR", "data": null}`))
			}

			if ok := validateJwt(parts[1]); ok {
				// Save client, removing them if logged-in already
				removeClient(conn)
				clients = append(clients, conn)
				loggedIn = true

				conn.Write(context.Background(), websocket.MessageText, []byte(`{"type": "AUTH_OK", "data": null}`))
			} else {
				conn.Write(context.Background(), websocket.MessageText, []byte(`{"type": "AUTH_ERROR", "data": null}`))
			}
		}
	}

	return nil
}

func SendSocketMessage(msgtype string, data interface{}) {
	json_string, err := json.Marshal(map[string]interface{}{"type": msgtype, "data": data})
	if err != nil {
		return
	}

	go func() {
		for _, conn := range clients {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			conn.Write(ctx, websocket.MessageText, []byte(json_string))
		}
	}()
}

func removeClient(c *websocket.Conn) {
	for index, client := range clients {
		if client == c {
			clients = append(clients[:index], clients[index+1:]...)
		}
	}
}
