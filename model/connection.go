package model

import (
	jsoniter "github.com/json-iterator/go"
	"goModel/message"
	"goModel/rawConn"
	"sync"
)

type Connection struct {
	m           *Model
	writeLock   sync.Mutex              // 写入锁, 保护 raw
	raw         rawConn.RawConn         // 原始连接
	msgHandlers map[string]func([]byte) // 报文处理函数
	statesLock  sync.RWMutex            // 保护 pubStates
	pubStates   map[string]struct{}     // 发布状态列表
	eventsLock  sync.RWMutex            // 保护 pubEvents
	pubEvents   map[string]struct{}     // 发布事件列表
}

func newConn(m *Model, raw rawConn.RawConn) *Connection {
	ans := &Connection{
		m:         m,
		raw:       raw,
		pubStates: make(map[string]struct{}),
		pubEvents: make(map[string]struct{}),
	}

	ans.msgHandlers = map[string]func([]byte){
		"set-subscribe-state":    ans.onSetSubState,
		"add-subscribe-state":    ans.onAddSubState,
		"remove-subscribe-state": ans.onRemoveSubState,
		"clear-subscribe-state":  ans.onClearSubState,
		"set-subscribe-event":    ans.onSetSubEvent,
		"add-subscribe-event":    ans.onAddSubEvent,
		"remove-subscribe-event": ans.onRemoveSubEvent,
		"clear-subscribe-event":  ans.onClearSubEvent,
		"state":                  ans.onState,
		"event":                  ans.onEvent,
		"call":                   ans.onCall,
		"response":               ans.onResp,
		"query-meta":             ans.onQueryMeta,
		"meta-info":              ans.onMetaInfo,
	}

	return ans
}

func (conn *Connection) dealReceive() {
	var closeReason string
	for {
		data, err := conn.raw.ReadMsg()
		if err != nil {
			closeReason = err.Error()
			break
		}

		msg := message.RawMessage{}
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal(data, &msg)
		if err != nil {
			closeReason = "decode json message failed"
			break
		}

		if handler, seen := conn.msgHandlers[msg.Type]; seen {
			handler(msg.Payload)
		}

	}

	conn.close(closeReason)
}

func (conn *Connection) close(reason string) {

}

func (conn *Connection) onSetSubState(payload []byte) {

}

func (conn *Connection) onAddSubState(payload []byte) {

}

func (conn *Connection) onRemoveSubState(payload []byte) {

}

func (conn *Connection) onClearSubState(payload []byte) {

}

func (conn *Connection) onSetSubEvent(payload []byte) {

}

func (conn *Connection) onAddSubEvent(payload []byte) {

}

func (conn *Connection) onRemoveSubEvent(payload []byte) {

}

func (conn *Connection) onClearSubEvent(payload []byte) {

}

func (conn *Connection) onState(payload []byte) {

}

func (conn *Connection) onEvent(payload []byte) {

}

func (conn *Connection) onCall(payload []byte) {

}

func (conn *Connection) onResp(payload []byte) {

}

func (conn *Connection) onQueryMeta(payload []byte) {

}

func (conn *Connection) onMetaInfo(payload []byte) {

}

func (conn *Connection) sendState(fullName string, data interface{}) {
	conn.statesLock.RLock()
	defer conn.statesLock.RUnlock()
	if _, seen := conn.pubStates[fullName]; seen {
		if msg, err := message.EncodeStateMsg(fullName, data); err == nil {
			_ = conn.sendMsg(msg)
		}
	}
}

func (conn *Connection) sendEvent(fullName string, args message.Args) {
	conn.eventsLock.RLock()
	defer conn.eventsLock.RUnlock()
	if _, seen := conn.pubStates[fullName]; seen {
		if msg, err := message.EncodeEventMsg(fullName, args); err == nil {
			_ = conn.sendMsg(msg)
		}
	}
}

func (conn *Connection) sendMsg(msg []byte) error {
	conn.writeLock.Lock()
	ans := conn.raw.WriteMsg(msg)
	conn.writeLock.Unlock()
	return ans
}
