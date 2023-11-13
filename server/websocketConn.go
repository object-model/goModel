package server

import "github.com/gorilla/websocket"

type webSocketConn struct {
	*websocket.Conn
}

func (conn *webSocketConn) ReadMsg() ([]byte, error) {
	messageType, p, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if messageType == websocket.BinaryMessage {
		return nil, nil
	}

	return p, nil
}

func (conn *webSocketConn) WriteMsg(msg []byte) error {
	return conn.WriteMessage(websocket.TextMessage, msg)
}

func NewWebSocketConn(conn *websocket.Conn) ModelConn {
	return &webSocketConn{conn}
}
