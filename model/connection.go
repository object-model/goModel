package model

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"goModel/message"
	"goModel/meta"
	"goModel/rawConn"
	"strings"
	"sync"
	"time"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

// StateHandler 状态报文处理接口
type StateHandler interface {
	OnState(modelName string, stateName string, data []byte)
}

// EventHandler 事件报文处理接口
type EventHandler interface {
	OnEvent(modelName string, eventName string, args message.RawArgs)
}

// ClosedHandler 连接关闭处理接口
type ClosedHandler interface {
	OnClosed(reason string)
}

// StateFunc 为状态回调函数, 参数modelName为状态报文对应的物模型名称,
// stateName 为状态报文对应的状态名, 参数data为状态数据.
type StateFunc func(modelName string, stateName string, data []byte)

func (s StateFunc) OnState(modelName string, stateName string, data []byte) {
	s(modelName, stateName, data)
}

// EventFunc 为事件回调函数, 参数modelName为事件报文对应的物模型名称,
// 参数eventName 为事件报文对应的状态名, 参数args为事件所携带参数.
type EventFunc func(modelName string, eventName string, args message.RawArgs)

func (e EventFunc) OnEvent(modelName string, eventName string, args message.RawArgs) {
	e(modelName, eventName, args)
}

// RespFunc 为响应回调函数, 参数resp为响应原始数据, 参数err为响应错误信息
type RespFunc func(resp message.RawResp, err error)

// ClosedFunc 为连接关闭回调函数, 参数modelName为关闭原因
type ClosedFunc func(reason string)

func (c ClosedFunc) OnClosed(reason string) {
	c(reason)
}

// Connection 为物模型连接,可以通过连接订阅状态和事件、注册状态和事件回调、远程调用方法、查询对端元信息.
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
	stateHandler    StateHandler              // 状态处理回调
	eventHandler    EventHandler              // 事件处理回调
	closedOnce      sync.Once                 // 确保 closedHandler 只调用一次
	closedHandler   ClosedHandler             // 连接关闭处理函数
	onMetaOnce      sync.Once                 // 确保只响应元信息报文一次
	metaGotCh       chan struct{}             // 对端元信息已获取信号
	peerMeta        *meta.Meta                // 对端的元信息
	peerMetaErr     error                     // 查询对端元信息的错误
	waitersLock     sync.Mutex                // 保护 respWaiters
	respWaiters     map[string]*RespWaiter    // 所有未收到响应的调用等待器
	uidCreator      func() string             // uuid生成器
}

// ConnOption 为创建连接选项
type ConnOption func(*Connection)

// WithStateHandler 配置连接的状态报文回调处理对象
func WithStateHandler(onState StateHandler) ConnOption {
	return func(connection *Connection) {
		if onState != nil {
			connection.stateHandler = onState
		}
	}
}

// WithStateFunc 配置连接的状态报文回调函数
func WithStateFunc(onState StateFunc) ConnOption {
	return func(connection *Connection) {
		if onState != nil {
			connection.stateHandler = onState
		}
	}
}

// WithEventFunc 配置连接的事件报文回调对象
func WithEventHandler(onEvent EventHandler) ConnOption {
	return func(connection *Connection) {
		if onEvent != nil {
			connection.eventHandler = onEvent
		}
	}
}

// WithEventFunc 配置连接的事件报文回调函数
func WithEventFunc(onEvent EventFunc) ConnOption {
	return func(connection *Connection) {
		if onEvent != nil {
			connection.eventHandler = onEvent
		}
	}
}

// WithClosedHandler 配置连接的关闭回调对象
func WithClosedHandler(onClose ClosedHandler) ConnOption {
	return func(connection *Connection) {
		if onClose != nil {
			connection.closedHandler = onClose
		}
	}
}

// WithClosedFunc 配置连接的关闭回调函数
func WithClosedFunc(onClose ClosedFunc) ConnOption {
	return func(connection *Connection) {
		if onClose != nil {
			connection.closedHandler = onClose
		}
	}
}

// WithStateBuffSize 配置连接的状态管道的大小
func WithStateBuffSize(size int) ConnOption {
	return func(connection *Connection) {
		if size > 0 {
			connection.statesChan = make(chan message.StatePayload, size)
		}
	}
}

// WithEventBuffSize 配置连接的事件管道的大小
func WithEventBuffSize(size int) ConnOption {
	return func(connection *Connection) {
		if size > 0 {
			connection.eventsChan = make(chan message.EventPayload, size)
		}
	}
}

func newConn(m *Model, raw rawConn.RawConn, opts ...ConnOption) *Connection {
	ans := &Connection{
		m:             m,
		raw:           raw,
		pubStates:     make(map[string]struct{}),
		pubEvents:     make(map[string]struct{}),
		statesChan:    make(chan message.StatePayload, 256),
		eventsChan:    make(chan message.EventPayload, 256),
		statesQuited:  make(chan struct{}),
		eventsQuited:  make(chan struct{}),
		stateHandler:  StateFunc(func(string, string, []byte) {}),
		eventHandler:  EventFunc(func(string, string, message.RawArgs) {}),
		closedHandler: ClosedFunc(func(string) {}),
		metaGotCh:     make(chan struct{}),
		peerMeta:      meta.NewEmptyMeta(),
		peerMetaErr:   fmt.Errorf("have NOT got peer meta yet"),
		respWaiters:   make(map[string]*RespWaiter),
		uidCreator:    uuid.NewString,
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

	for _, option := range opts {
		option(ans)
	}

	go ans.dealState()
	go ans.dealEvent()

	return ans
}

// SubState 通过连接conn发送状态订阅报文,订阅状态列表states中的所有状态,并返回错误信息.
func (conn *Connection) SubState(states []string) error {
	msg := message.Must(message.EncodeSubStateMsg(message.SetSub, states))
	return conn.sendMsg(msg)
}

// AddSubState 通过连接conn发送添加状态订阅报文,新增对状态列表states中的所有状态的订阅,并返回错误信息.
func (conn *Connection) AddSubState(states []string) error {
	msg := message.Must(message.EncodeSubStateMsg(message.AddSub, states))
	return conn.sendMsg(msg)
}

// CancelSubState 通过连接conn发送取消状态订阅报文,取消对状态列表states中所有状态的订阅,并返回错误信息.
func (conn *Connection) CancelSubState(states []string) error {
	msg := message.Must(message.EncodeSubStateMsg(message.RemoveSub, states))
	return conn.sendMsg(msg)
}

// CancelAllSubState 通过连接conn发送取消所有状态订阅报文,取消对所有状态的订阅,并返回错误信息.
func (conn *Connection) CancelAllSubState() error {
	msg := message.Must(message.EncodeSubStateMsg(message.RemoveSub, nil))
	return conn.sendMsg(msg)
}

// SubEvent 通过连接conn发送事件订阅报文,订阅事件列表events中所有事件,并返回错误信息.
func (conn *Connection) SubEvent(events []string) error {
	msg := message.Must(message.EncodeSubEventMsg(message.SetSub, events))
	return conn.sendMsg(msg)
}

// AddSubEvent 通过连接conn发送添加事件订阅报文,新增对事件列表events中所有事件的订阅,并返回错误信息.
func (conn *Connection) AddSubEvent(events []string) error {
	msg := message.Must(message.EncodeSubEventMsg(message.AddSub, events))
	return conn.sendMsg(msg)
}

// CancelSubEvent 通过连接conn发送取消事件订阅报文,取消对事件列表events中所有事件的订阅,并返回错误信息.
func (conn *Connection) CancelSubEvent(events []string) error {
	msg := message.Must(message.EncodeSubEventMsg(message.RemoveSub, events))
	return conn.sendMsg(msg)
}

// CancelAllSubEvent 通过连接conn发送取消所有事件订阅报文,取消对所有事件的订阅,并返回错误信息.
func (conn *Connection) CancelAllSubEvent() error {
	msg := message.Must(message.EncodeSubEventMsg(message.RemoveSub, nil))
	return conn.sendMsg(msg)
}

// Invoke 通过连接conn发送调用请求报文,以异步的方式远程调用名为fullName的方法,调用参数为args,
// 返回用于等待该次调用的响应的等待对象和错误信息. 出错时该函数返回的等待对象为nil.
func (conn *Connection) Invoke(fullName string, args message.Args) (*RespWaiter, error) {
	uid := conn.uidCreator()
	msg, err := message.EncodeCallMsg(fullName, uid, args)
	if err != nil {
		return nil, err
	}
	waiter := conn.addRespWaiter(uid)
	if err = conn.sendMsg(msg); err != nil {
		conn.removeRespWaiter(uid)
		return nil, err
	}

	return waiter, nil
}

// InvokeByCallback 异步调用名为fullName的方法,调用参数为args,当收到对应的响应报文时会调用onResp.
// 若该函数返回的错误信息不为nil, 则表示调用请求发送失败, 回调onResp不会被触发.
func (conn *Connection) InvokeByCallback(fullName string, args message.Args, onResp RespFunc) error {
	waiter, err := conn.Invoke(fullName, args)
	if err != nil {
		return err
	}

	if onResp != nil {
		go func() {
			onResp(waiter.Wait())
		}()
	}

	return nil
}

// InvokeFor 异步调用名为fullName的方法,调用参数为args,当收到对应的响应报文时会调用onResp.
// 若该函数返回的错误信息不为nil, 则表示调用请求发送失败, 回调onResp不会被触发.
// InvokeFor 与 InvokeByCallback 的区别是, InvokeFor 在后台等待响应报文时,有超时时间为timeout的限制,
// 若在timeout时间内未收到对应的响应报文,则会调用onResp,调用返回值为空,错误信息为超时.
func (conn *Connection) InvokeFor(fullName string, args message.Args, onResp RespFunc, timeout time.Duration) error {
	waiter, err := conn.Invoke(fullName, args)
	if err != nil {
		return err
	}

	if onResp != nil {
		go func() {
			onResp(waiter.WaitFor(timeout))
		}()
	}

	return nil
}

// Call 通过连接conn发送调用请求报文,以同步的方式远程调用名为fullName的方法,调用参数为args,等待调用响应报文的返回.
// Call 在成功发送调用请求报文后会一直等待,直到收到调用响应报文或者连接关闭再返回.
func (conn *Connection) Call(fullName string, args message.Args) (message.RawResp, error) {
	waiter, err := conn.Invoke(fullName, args)
	if err != nil {
		return message.RawResp{}, err
	}

	return waiter.Wait()
}

// CallFor 通过连接conn发送调用请求报文,以同步的方式远程调用名为fullName的方法,调用参数为args,等待调用响应报文的返回.
// CallFor 和 Call 类似, 都会阻塞式地等待调用响应报文, 只不过 CallFor 有等待超时时间为timeout的限制.
func (conn *Connection) CallFor(fullName string, args message.Args, timeout time.Duration) (message.RawResp, error) {
	waiter, err := conn.Invoke(fullName, args)
	if err != nil {
		return message.RawResp{}, err
	}
	return waiter.WaitFor(timeout)
}

// GetPeerMeta 阻塞式地获取对端的元信息,若先前已经收到对端的元信息报文,则直接返回不再发送查询元信息报文.
// 该函数会阻塞式地等待, 直到收到对端元信息或者连接关闭.
func (conn *Connection) GetPeerMeta() (*meta.Meta, error) {
	select {
	case <-conn.metaGotCh:
		return conn.peerMeta, conn.peerMetaErr
	default:
		err := conn.sendMsg(message.EncodeQueryMetaMsg())
		if err != nil {
			return conn.peerMeta, err
		}
		<-conn.metaGotCh
		return conn.peerMeta, conn.peerMetaErr
	}
}

// Close 关闭连接.
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
			reason = fmt.Sprintf("decode json: %s", err.Error())
			break
		}

		if handler, seen := conn.msgHandlers[msg.Type]; seen {
			handler(msg.Payload)
		}

	}
}

func (conn *Connection) close(reason string) error {
	// NOTE: 关闭前需要唤醒所有等待者, 避免不必要的等待
	conn.notifyRespWaiterOnClose(reason)
	conn.notifyMetaWaiterOnClose(reason)

	err := conn.raw.Close()

	// 调用关闭回调
	conn.closedOnce.Do(func() {
		conn.closedHandler.OnClosed(reason)
	})
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

	// 字段缺失或者为空
	if strings.TrimSpace(state.Name) == "" || state.Data == nil {
		return
	}

	conn.statesChan <- state
}

func (conn *Connection) onEvent(payload []byte) {
	event := message.EventPayload{}
	if json.Unmarshal(payload, &event) != nil {
		return
	}

	// 字段缺失或者为空
	if strings.TrimSpace(event.Name) == "" || event.Args == nil {
		return
	}

	conn.eventsChan <- event
}

func (conn *Connection) onCall(payload []byte) {
	call := message.CallPayload{}
	if json.Unmarshal(payload, &call) != nil {
		return
	}
	// 参数缺失或者为空
	if strings.TrimSpace(call.Name) == "" ||
		strings.TrimSpace(call.UUID) == "" ||
		call.Args == nil {
		return
	}
	go conn.dealCallReq(call)
}

func (conn *Connection) onResp(payload []byte) {
	resp := message.ResponsePayload{}
	if json.Unmarshal(payload, &resp) != nil {
		return
	}

	// 参数缺失或者为空
	// NOTE: 无error字段, 认为无错误, 不视为出错
	if strings.TrimSpace(resp.UUID) == "" || resp.Response == nil {
		return
	}

	waiter := conn.removeRespWaiter(resp.UUID)
	if waiter == nil {
		return
	}

	// error字段为空，则认为没出错
	var err error = nil
	if errStr := strings.TrimSpace(resp.Error); errStr != "" {
		err = errors.New(errStr)
	}

	// 唤醒等待
	waiter.wake(resp.Response, err)
}

func (conn *Connection) onQueryMeta([]byte) {
	msg := message.Must(message.EncodeRawMsg("meta-info", conn.m.meta.ToJSON()))
	_ = conn.sendMsg(msg)
}

func (conn *Connection) onMetaInfo(payload []byte) {
	conn.onMetaOnce.Do(func() {
		conn.peerMeta, conn.peerMetaErr = meta.Parse(payload, nil)
		close(conn.metaGotCh)
	})
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
	if _, seen := conn.pubEvents[fullName]; seen {
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

		conn.stateHandler.OnState(modelName, stateName, state.Data)
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

		conn.eventHandler.OnEvent(modelName, eventName, event.Args)
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
	resp := conn.m.callReqHandler.OnCallReq(methodName, args)
	if resp == nil {
		resp = message.Resp{}
	}

	// 7.校验响应
	errStr := ""
	if conn.m.verifyResp {
		err := conn.m.meta.VerifyMethodResp(methodName, resp)
		if err != nil {
			errStr = err.Error()
		}
	}

	// 8.发送响应
	msg := message.Must(message.EncodeRespMsg(uuidStr,
		errStr,
		resp))

	// TODO: 发送失败是否需要写日志
	_ = conn.sendMsg(msg)
}

func (conn *Connection) addRespWaiter(uuid string) *RespWaiter {
	conn.waitersLock.Lock()
	defer conn.waitersLock.Unlock()
	waiter := &RespWaiter{
		got: make(chan struct{}),
	}
	conn.respWaiters[uuid] = waiter
	return waiter
}

func (conn *Connection) removeRespWaiter(uuid string) *RespWaiter {
	conn.waitersLock.Lock()
	defer conn.waitersLock.Unlock()
	waiter := conn.respWaiters[uuid]
	return waiter
}

func (conn *Connection) notifyRespWaiterOnClose(reason string) {
	conn.waitersLock.Lock()
	defer conn.waitersLock.Unlock()

	// 唤醒所有等待
	for _, waiter := range conn.respWaiters {
		waiter.wake(message.RawResp{}, fmt.Errorf("connection closed for: %s", reason))
	}

	// 清空等待对象
	conn.respWaiters = make(map[string]*RespWaiter)
}

func (conn *Connection) notifyMetaWaiterOnClose(reason string) {
	conn.onMetaOnce.Do(func() {
		conn.peerMetaErr = fmt.Errorf("connection closed for: %s", reason)
		close(conn.metaGotCh)
	})
}
