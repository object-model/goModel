package model

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"goModel/message"
	"goModel/rawConn"
	"strings"
	"sync"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// StateFunc 为状态回调函数, 参数modelName为状态报文对应的物模型名称,
// stateName 为状态报文对应的状态名, 参数data为状态数据.
type StateFunc func(modelName string, stateName string, data []byte)

// EventFunc 为事件回调函数, 参数modelName为事件报文对应的物模型名称,
// 参数eventName 为事件报文对应的状态名, 参数args为事件所携带参数.
type EventFunc func(modelName string, eventName string, args message.RawArgs)

type Connection struct {
	m               *Model
	writeLock       sync.Mutex                // 写入锁, 保护 raw
	raw             rawConn.RawConn           // 原始连接
	msgHandlers     map[string]func([]byte)   // 报文处理函数
	statesLock      sync.RWMutex              // 保护 pubStates
	pubStates       map[string]struct{}       // 发布状态列表
	eventsLock      sync.RWMutex              // 保护 pubEvents
	pubEvents       map[string]struct{}       // 发布事件列表
	statesCloseOnce sync.Once                 // 确保 statesChan 只关闭一次
	statesChan      chan message.StatePayload // 状态管道
	statesQuited    chan struct{}             // dealState 完全退出信号
	eventsCloseOnce sync.Once                 // 确保 eventsChan 只关闭一次
	eventsChan      chan message.EventPayload // 事件管道
	eventsQuited    chan struct{}             // dealEvent 完全退出信号
	stateHandler    StateFunc                 // 状态处理回调
	eventHandler    EventFunc                 // 事件处理回调
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
		statesQuited: make(chan struct{}),
		eventsQuited: make(chan struct{}),
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
	go ans.dealEvent()

	return ans
}

func (conn *Connection) Close() error {
	return conn.raw.Close()
}

func (conn *Connection) dealReceive() {
	defer func() {
		_ = conn.Close()

		conn.statesCloseOnce.Do(func() {
			close(conn.statesChan)
		})
		conn.eventsCloseOnce.Do(func() {
			close(conn.eventsChan)
		})
		<-conn.statesQuited
		<-conn.eventsQuited
	}()

	for {
		data, err := conn.raw.ReadMsg()
		if err != nil {
			break
		}

		msg := message.RawMessage{}
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal(data, &msg)
		if err != nil {
			break
		}

		if handler, seen := conn.msgHandlers[msg.Type]; seen {
			handler(msg.Payload)
		}

	}
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
	if json.Unmarshal(payload, &state) != nil {
		return
	}
	conn.statesChan <- state
}

func (conn *Connection) onEvent(payload []byte) {
	event := message.EventPayload{}
	if json.Unmarshal(payload, &event) != nil {
		return
	}
	conn.eventsChan <- event
}

func (conn *Connection) onCall(payload []byte) {
	call := message.CallPayload{}
	if json.Unmarshal(payload, &call) != nil {
		return
	}
	go conn.dealCallReq(call)
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
	defer close(conn.statesQuited)
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
	defer close(conn.eventsQuited)
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

func (conn *Connection) dealCallReq(call message.CallPayload) {
	// 1.获取调用参数信息
	fullName := call.Name
	uuidStr := call.UUID
	args := call.Args

	// 2.分解模型名和方法名
	i := strings.LastIndex(fullName, "/")
	if i == -1 {
		resp := message.Must(message.EncodeRespMsg(uuidStr,
			"fullName is invalid format",
			message.Resp{}))
		_ = conn.sendMsg(resp)
		return
	}

	modelName := fullName[:i]
	methodName := fullName[i+1:]

	// 3.校验模型名称是否匹配
	if modelName != conn.m.meta.Name {
		resp := message.Must(message.EncodeRespMsg(uuidStr,
			fmt.Sprintf("modelName %q: unmatched", modelName),
			message.Resp{}))
		_ = conn.sendMsg(resp)
		return
	}

	// 4. 校验调用请求参数
	if err := conn.m.meta.VerifyRawMethodArgs(methodName, args); err != nil {
		resp := message.Must(message.EncodeRespMsg(uuidStr,
			err.Error(),
			message.Resp{}))
		_ = conn.sendMsg(resp)
		return
	}

	// 5.没有注册回调，直接返回错误信息
	if conn.m.callReqHandler == nil {
		resp := message.Must(message.EncodeRespMsg(uuidStr,
			"NO callback",
			message.Resp{}))
		_ = conn.sendMsg(resp)
		return
	}

	// 6.调用回调
	resp := conn.m.callReqHandler(methodName, args)

	// 7.校验响应
	// TODO: 添加开关，控制是否开启响应校验
	errStr := ""
	err := conn.m.meta.VerifyMethodResp(methodName, resp)
	if err != nil {
		errStr = err.Error()
	}

	// 8.发送响应
	msg := message.Must(message.EncodeRespMsg(uuidStr,
		errStr,
		resp))

	if conn.sendMsg(msg) != nil {
		// TODO: 发送失败是否需要写日志
	}
}
