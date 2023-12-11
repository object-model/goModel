package model

import (
	jsoniter "github.com/json-iterator/go"
	"goModel/message"
	"goModel/rawConn"
	"strings"
	"sync"
)

// StateFunc 为状态回调函数, 参数modelName为状态报文对应的物模型名称,
// stateName 为状态报文对应的状态名, 参数data为状态数据.
type StateFunc func(modelName string, stateName string, data []byte)

// EventFunc 为事件回调函数, 参数modelName为事件报文对应的物模型名称,
// 参数eventName 为事件报文对应的状态名, 参数args为事件所携带参数.
type EventFunc func(modelName string, eventName string, args message.RawArgs)

type Connection struct {
	m            *Model
	writeLock    sync.Mutex                // 写入锁, 保护 raw
	raw          rawConn.RawConn           // 原始连接
	msgHandlers  map[string]func([]byte)   // 报文处理函数
	statesLock   sync.RWMutex              // 保护 pubStates
	pubStates    map[string]struct{}       // 发布状态列表
	eventsLock   sync.RWMutex              // 保护 pubEvents
	pubEvents    map[string]struct{}       // 发布事件列表
	statesChan   chan message.StatePayload // 状态管道
	eventsChan   chan message.EventPayload // 事件管道
	stateHandler StateFunc                 // 状态处理回调
	eventHandler EventFunc                 // 事件处理回调
}

func newConn(m *Model, raw rawConn.RawConn, stateFunc StateFunc, eventFunc EventFunc) *Connection {
	if stateFunc == nil {
		stateFunc = func(string, string, []byte) {}
	}

	if eventFunc == nil {
		eventFunc = func(string, string, message.RawArgs) {}
	}

	ans := &Connection{
		m:            m,
		raw:          raw,
		pubStates:    make(map[string]struct{}),
		pubEvents:    make(map[string]struct{}),
		statesChan:   make(chan message.StatePayload, 256),
		eventsChan:   make(chan message.EventPayload, 256),
		stateHandler: stateFunc,
		eventHandler: eventFunc,
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

	go ans.dealState()

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
	state := message.StatePayload{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if json.Unmarshal(payload, &state) != nil {
		return
	}
	conn.statesChan <- state
}

func (conn *Connection) onEvent(payload []byte) {
	event := message.EventPayload{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if json.Unmarshal(payload, &event) != nil {
		return
	}
	conn.eventsChan <- event
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

func (conn *Connection) dealState() {
	for state := range conn.statesChan {
		i := strings.LastIndex(state.Name, "/")
		if i == -1 {
			continue
		}
		modelName := state.Name[:i]
		stateName := state.Name[i+1:]

		conn.stateHandler(modelName, stateName, state.Data)
	}
}

func (conn *Connection) dealEvent() {
	for event := range conn.eventsChan {
		i := strings.LastIndex(event.Name, "/")
		if i == -1 {
			continue
		}
		modelName := event.Name[:i]
		eventName := event.Name[i+1:]

		conn.eventHandler(modelName, eventName, event.Args)
	}
}
