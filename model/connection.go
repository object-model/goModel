package model

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"goModel/message"
	"goModel/meta"
	"goModel/rawConn"
	"strings"
	"sync"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

// StateFunc 为状态回调函数, 参数modelName为状态报文对应的物模型名称,
// stateName 为状态报文对应的状态名, 参数data为状态数据.
type StateFunc func(modelName string, stateName string, data []byte)

// EventFunc 为事件回调函数, 参数modelName为事件报文对应的物模型名称,
// 参数eventName 为事件报文对应的状态名, 参数args为事件所携带参数.
type EventFunc func(modelName string, eventName string, args message.RawArgs)

// ClosedFunc 为连接关闭回调函数, 参数modelName为关闭原因
type ClosedFunc func(reason string)

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
	closedHandler   ClosedFunc                // 连接关闭处理函数
	onMetaOnce      sync.Once                 // 确保只响应元信息报文一次
	metaGotCh       chan struct{}             // 对端元信息已获取信号
	peerMeta        *meta.Meta                // 对端的元信息
	peerMetaErr     error                     // 查询对端元信息的错误
}

type ConnCallback struct {
	OnState  StateFunc
	OnEvent  EventFunc
	OnClosed ClosedFunc
}

func newConn(m *Model, raw rawConn.RawConn, callBack ConnCallback) *Connection {
	if callBack.OnState == nil {
		callBack.OnState = func(string, string, []byte) {}
	}

	if callBack.OnEvent == nil {
		callBack.OnEvent = func(string, string, message.RawArgs) {}
	}

	if callBack.OnClosed == nil {
		callBack.OnClosed = func(string) {}
	}

	ans := &Connection{
		m:             m,
		raw:           raw,
		pubStates:     make(map[string]struct{}),
		pubEvents:     make(map[string]struct{}),
		statesChan:    make(chan message.StatePayload, 256),
		eventsChan:    make(chan message.EventPayload, 256),
		statesQuited:  make(chan struct{}),
		eventsQuited:  make(chan struct{}),
		stateHandler:  callBack.OnState,
		eventHandler:  callBack.OnEvent,
		closedHandler: callBack.OnClosed,
		metaGotCh:     make(chan struct{}),
		peerMeta:      meta.NewEmptyMeta(),
		peerMetaErr:   fmt.Errorf("have NOT got peer meta yet"),
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

func (conn *Connection) GetPeerMeta() (*meta.Meta, error) {
	select {
	case <-conn.metaGotCh:
		return conn.peerMeta, conn.peerMetaErr
	default:
		err := conn.sendMsg(message.EncodeQueryMetaMsg())
		if err != nil {
			return conn.peerMeta, fmt.Errorf("send query-meta message failed")
		}
		<-conn.metaGotCh
		return conn.peerMeta, conn.peerMetaErr
	}
}

func (conn *Connection) Close() error {
	return conn.close("active close")
}

func (conn *Connection) dealReceive() {
	reason := ""
	defer func() {
		_ = conn.close(reason)

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
			reason = err.Error()
			break
		}

		msg := message.RawMessage{}
		err = json.Unmarshal(data, &msg)
		if err != nil {
			reason = err.Error()
			break
		}

		if handler, seen := conn.msgHandlers[msg.Type]; seen {
			handler(msg.Payload)
		}

	}
}

func (conn *Connection) close(reason string) error {
	err := conn.raw.Close()
	conn.closedHandler(reason)
	return err
}

func (conn *Connection) onSetSubState(payload []byte) {
	var states []string
	if err := json.Unmarshal(payload, &states); err != nil {
		return
	}

	ans := make(map[string]struct{})
	for _, state := range states {
		ans[state] = struct{}{}
	}

	conn.statesLock.Lock()
	conn.pubStates = ans
	conn.statesLock.Unlock()
}

func (conn *Connection) onAddSubState(payload []byte) {
	var states []string
	if err := json.Unmarshal(payload, &states); err != nil {
		return
	}

	conn.statesLock.Lock()
	for _, state := range states {
		conn.pubStates[state] = struct{}{}
	}
	conn.statesLock.Unlock()
}

func (conn *Connection) onRemoveSubState(payload []byte) {
	var states []string
	if err := json.Unmarshal(payload, &states); err != nil {
		return
	}

	conn.statesLock.Lock()
	for _, state := range states {
		delete(conn.pubStates, state)
	}
	conn.statesLock.Unlock()
}

func (conn *Connection) onClearSubState([]byte) {
	conn.statesLock.Lock()
	conn.pubStates = make(map[string]struct{})
	conn.statesLock.Unlock()
}

func (conn *Connection) onSetSubEvent(payload []byte) {
	var events []string
	if err := json.Unmarshal(payload, &events); err != nil {
		return
	}

	ans := make(map[string]struct{})
	for _, event := range events {
		ans[event] = struct{}{}
	}

	conn.eventsLock.Lock()
	conn.pubEvents = ans
	conn.eventsLock.Unlock()
}

func (conn *Connection) onAddSubEvent(payload []byte) {
	var events []string
	if err := json.Unmarshal(payload, &events); err != nil {
		return
	}

	conn.eventsLock.Lock()
	for _, event := range events {
		conn.pubEvents[event] = struct{}{}
	}
	conn.eventsLock.Unlock()
}

func (conn *Connection) onRemoveSubEvent(payload []byte) {
	var events []string
	if err := json.Unmarshal(payload, &events); err != nil {
		return
	}

	conn.eventsLock.Lock()
	for _, event := range events {
		delete(conn.pubEvents, event)
	}
	conn.eventsLock.Unlock()
}

func (conn *Connection) onClearSubEvent([]byte) {
	conn.eventsLock.Lock()
	conn.pubEvents = make(map[string]struct{})
	conn.eventsLock.Unlock()
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
	conn.onMetaOnce.Do(func() {
		conn.peerMeta, conn.peerMetaErr = meta.Parse(payload, nil)
	})
}

func (conn *Connection) onMetaInfo([]byte) {
	msg := message.Must(message.EncodeRawMsg("meta-info", conn.m.meta.ToJSON()))
	_ = conn.sendMsg(msg)
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
