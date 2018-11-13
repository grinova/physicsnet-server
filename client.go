package physicsnet

import "github.com/gorilla/websocket"

// Client - клиент
type Client struct {
	conn   *websocket.Conn
	except *except
}

// SendSystemMessage отправляет системное сообщение
func (c *Client) SendSystemMessage(v interface{}) {
	c.sync(message{Type: "system", Data: route{ID: "default", Data: v}})
}

func (c *Client) sync(v interface{}) {
	c.conn.WriteJSON(v)
}
