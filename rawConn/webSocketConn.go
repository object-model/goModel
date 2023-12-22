package rawConn

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

const (
	// 必须在此时间内收到pong报文
	pongWait = 20 * time.Second

	// 发送ping报文的周期
	pingPeriod = (pongWait * 9) / 10
)

type webSocketConn struct {
	writeMu sync.Mutex
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
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, msg)
}

func (conn *webSocketConn) writePing() error {
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()
	return conn.WriteMessage(websocket.PingMessage, nil)
}

func NewWebSocketConn(conn *websocket.Conn, ping bool) RawConn {
	ans := &webSocketConn{
		writeMu: sync.Mutex{},
		Conn:    conn,
	}

	if !ping {
		return ans
	}

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ans.writePing(); err != nil {
					return
				}
			}
		}
	}()

	return ans
}
