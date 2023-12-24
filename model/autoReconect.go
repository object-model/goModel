package model

import (
	"errors"
	"goModel/message"
	"goModel/meta"
	"sync"
	"time"
)

// OnReConnect 为重连回调函数, 参数num为重连次数,
// 参数ok为是否重连成功. OnReConnect 在每次发生重连事件后调用.
type OnReConnect func(num uint, ok bool)

type AutoConnector struct {
	*Connection                     // 连接
	mutex       sync.RWMutex        // 保护 conn, subStates, subEvents
	subStates   map[string]struct{} // 订阅的状态列表
	subEvents   map[string]struct{} // 订阅的事件列表
	exitOnce    sync.Once           // 保证仅退出重连一次
	exit        chan struct{}       // 是否退出重连
	m           *Model              // 物模型
	addr        string              // 连接的对端地址
	forever     bool                // 是否永久重连 (仅在首次连接成功后有效)
	maxTryNum   uint                // 最大重连次数
	onReconnect OnReConnect         // 重连回调函数
	onStop      func()              // 停止重连回调
	connOptions []ConnOption        // 连接选项
}

// AutoConnectorOption 为自动重连对象配置
type AutoConnectorOption func(a *AutoConnector)

// WithMaxTryNum 配置自动重连的最大次数为maxTryNum,
// 若参数maxTryNum为0, 则该配置无效.
// 若重连次数大于等于maxTryNum后,仍无法建立连接,则不再重连.
// 该配置会在 WithForever 存在的情况下无效.
func WithMaxTryNum(maxTryNum uint) AutoConnectorOption {
	return func(a *AutoConnector) {
		if maxTryNum > 0 {
			a.maxTryNum = maxTryNum
		}
	}
}

// WithForever 配置永久自动重连, 无论重连多少次都会一直重连, 直到恢复连接.
// 该配置会使 WithMaxTryNum 配置无效.
func WithForever() AutoConnectorOption {
	return func(a *AutoConnector) {
		a.forever = true
	}
}

func WithOnStop(onStop func()) AutoConnectorOption {
	return func(a *AutoConnector) {
		if onStop != nil {
			a.onStop = onStop
		}
	}
}

// WithOnReConnect 配置自动重连回调函数为onReConnect,
// 当发生自动重连事件后会触发onReConnect.
// 若参数onReConnect为空, 则该配置无效
func WithOnReConnect(onReConnect OnReConnect) AutoConnectorOption {
	return func(a *AutoConnector) {
		if onReConnect != nil {
			a.onReconnect = onReConnect
		}
	}
}

// WithConnOption 配置自动重连对象所包含连接的连接设置, 如状态回调和事件回调.
// AutoConnector 会覆盖 WithClosedHandler 和 WithClosedFunc 所配置的连接关闭处理逻辑.
func WithConnOption(connOption ...ConnOption) AutoConnectorOption {
	return func(a *AutoConnector) {
		a.connOptions = connOption
	}
}

// NewAutoConnector 会根据自动重连配置options创建一个自动重连对象,
// 对象创建后自动通过物模型m与地址为addr的服务端建立连接, 若连接建立成功后续连接断开自动触发重连.
// 默认不会永久重连, 最大重连次数为5次.
// 自动重连对象在自动重连成功后会恢复之前有效连接的状态和事件订阅关系.
// 每次重连, 自动重连对象会触发 WithOnReConnect 所配置的回调, 告知重连次数和是否重连成功.
func NewAutoConnector(m *Model, addr string, options ...AutoConnectorOption) *AutoConnector {
	ans := &AutoConnector{
		subStates:   make(map[string]struct{}),
		subEvents:   make(map[string]struct{}),
		exit:        make(chan struct{}),
		m:           m,
		addr:        addr,
		forever:     false,
		maxTryNum:   5,
		onStop:      func() {},
		onReconnect: func(uint, bool) {},
		connOptions: make([]ConnOption, 0, 4),
	}

	for _, option := range options {
		option(ans)
	}

	ans.connOptions = append(ans.connOptions, WithClosedHandler(ans))

	ans.setConn(ans.reconnect())

	return ans
}

func (a *AutoConnector) SubState(states []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.subStates = map[string]struct{}{}
	for _, state := range states {
		a.subStates[state] = struct{}{}
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.SubState(states)
}

func (a *AutoConnector) AddSubState(states []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, state := range states {
		a.subStates[state] = struct{}{}
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.AddSubState(states)
}

func (a *AutoConnector) CancelSubState(states []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, state := range states {
		delete(a.subStates, state)
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.CancelSubState(states)
}

func (a *AutoConnector) CancelAllSubState() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.subStates = make(map[string]struct{})
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.CancelAllSubState()
}

func (a *AutoConnector) SubEvent(events []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.subEvents = map[string]struct{}{}
	for _, event := range events {
		a.subEvents[event] = struct{}{}
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.SubEvent(events)
}

func (a *AutoConnector) AddSubEvent(events []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, event := range events {
		a.subEvents[event] = struct{}{}
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.AddSubEvent(events)
}

func (a *AutoConnector) CancelSubEvent(events []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, event := range events {
		delete(a.subEvents, event)
	}
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.CancelSubEvent(events)
}

func (a *AutoConnector) CancelAllSubEvent() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.subEvents = make(map[string]struct{})
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.CancelAllSubEvent()
}

func (a *AutoConnector) Invoke(fullName string, args message.Args) (*RespWaiter, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return nil, errors.New("nil connection")
	}
	return a.Connection.Invoke(fullName, args)
}

func (a *AutoConnector) InvokeByCallback(fullName string, args message.Args, onResp RespFunc) error {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.InvokeByCallback(fullName, args, onResp)
}

func (a *AutoConnector) InvokeFor(fullName string, args message.Args, onResp RespFunc, timeout time.Duration) error {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.InvokeFor(fullName, args, onResp, timeout)
}

func (a *AutoConnector) Call(fullName string, args message.Args) (message.RawResp, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return message.RawResp{}, errors.New("nil connection")
	}
	return a.Connection.Call(fullName, args)
}

func (a *AutoConnector) GetPeerMeta() (*meta.Meta, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return meta.NewEmptyMeta(), errors.New("nil connection")
	}
	return a.Connection.GetPeerMeta()
}

func (a *AutoConnector) CallFor(fullName string, args message.Args, timeout time.Duration) (message.RawResp, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.Connection == nil {
		return message.RawResp{}, errors.New("nil connection")
	}
	return a.Connection.CallFor(fullName, args, timeout)
}

func (a *AutoConnector) Close() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.exitOnce.Do(func() {
		close(a.exit)
	})
	if a.Connection == nil {
		return errors.New("nil connection")
	}
	return a.Connection.Close()
}

func (a *AutoConnector) OnClosed(string) {
	a.setConn(a.reconnect())
}

func (a *AutoConnector) isExit() bool {
	select {
	case <-a.exit:
		return true
	default:
		return false
	}
}

func (a *AutoConnector) reconnect() *Connection {
	for i := uint(0); !a.isExit(); {
		i++
		conn, err := a.m.Dial(a.addr, a.connOptions...)
		a.onReconnect(i, err == nil)
		if err == nil {
			return conn
		}

		if a.forever {
			continue
		}
		if i >= a.maxTryNum {
			break
		}
	}
	a.onStop()
	return nil
}

func (a *AutoConnector) setConn(connection *Connection) {
	a.mutex.Lock()
	a.Connection = connection
	if connection == nil {
		// 连接建立成功要恢复之前状态和事件订阅
		_ = a.Connection.SubEvent(set2slice(a.subEvents))
		_ = a.Connection.SubState(set2slice(a.subStates))
	}
	a.mutex.Unlock()
}

func set2slice(set map[string]struct{}) []string {
	res := make([]string, 0, len(set))
	for item := range set {
		res = append(res, item)
	}
	return res
}
