package server

import (
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/object-model/goModel/message"
	"github.com/object-model/goModel/meta"
	"github.com/object-model/goModel/rawConn"
	"log"
	"strings"
	"sync"
	"time"
)

type msgPack struct {
	Type     string
	payload  []byte
	fullData []byte
}

type msgHandler func(msg msgPack) error

type stateOrEventMessage struct {
	Name     string // 状态或者事件名称
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type callMessage struct {
	Source   string                         // 调用者的模型名
	Model    string                         // 调用目标的模型名
	Method   string                         // 调用目标的方法名
	UUID     string                         // 调用UUID
	Args     map[string]jsoniter.RawMessage // 调用参数
	FullData []byte                         // 全报文原始数据，是Message类型序列化的结果
}

type responseMessage struct {
	Source   string // 发送响应报文的模型名
	UUID     string // 调用UUID
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type subStateOrEventMessage struct {
	Source string   // 发送者的物模型名称
	Type   int      // 订阅类型
	Items  []string // 状态或者事件列表
}

type model struct {
	rawConn.RawConn                               // 原始连接
	writerQuit      chan struct{}                 // 退出 writer 的信号
	added           chan struct{}                 // 连接已经加入 Server 信号
	removeConnCh    chan<- *model                 // 删除连接通道
	stateBroadcast  chan<- stateOrEventMessage    // 状态广播通道
	eventBroadcast  chan<- stateOrEventMessage    // 事件广播通道
	callChan        chan<- callMessage            // 调用请求通道
	respChan        chan<- responseMessage        // 响应结果通道
	subStateChan    chan<- subStateOrEventMessage // 更新状态订阅写入通道
	subEventChan    chan<- subStateOrEventMessage // 更新事件订阅写入通道
	writeChan       chan []byte                   // 数据写入通道
	metaGotChan     chan struct{}                 // 收到元信息消息通道
	queryOnce       sync.Once                     // 保证只查询一次元信息
	onGetMetaOnce   sync.Once                     // 保证只响应一次元信息结果报文
	quitWriterOnce  sync.Once                     // 保证 writerQuit 只关闭一次
	addedOnce       sync.Once                     // 保证 added 只关闭一次
	MetaInfo        *meta.Meta                    // 元信息
	MetaRaw         []byte                        // 原始的元信息
	log             *log.Logger                   // 记录收发数据
	buffer          []msgPack                     // 挂起的报文
	closeReason     string                        // 连接关闭原因
	msgHandlers     map[string]msgHandler         // 报文消息处理函数集合
}

func (m *model) quitWriter() {
	m.quitWriterOnce.Do(func() {
		close(m.writerQuit)
	})
}

func (m *model) setAdded() {
	m.addedOnce.Do(func() {
		close(m.added)
	})
}

func (m *model) isAdded() bool {
	select {
	case <-m.added:
		return true
	default:
		return false
	}
}

func (m *model) queryMeta(timeout time.Duration) error {
	m.queryOnce.Do(func() {
		m.writeChan <- message.EncodeQueryMetaMsg()
	})

	select {
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	case <-m.metaGotChan:
		break
	}

	return nil
}

func (m *model) reader() {
	defer func() {
		// 推送连接关闭事件
		m.notifyClosed()

		// 通过Server退出writer
		m.removeConnCh <- m
	}()
	for {
		// 读取报文
		data, err := m.ReadMsg()
		if err != nil {
			m.closeReason = err.Error()
			break
		}

		if len(data) <= 0 {
			continue
		}

		// 记录接收数据
		m.log.Println("<--", m.RemoteAddr().String(), string(data))

		// 解析JSON报文
		rawMessage := message.RawMessage{}
		if err = jsoniter.Unmarshal(data, &rawMessage); err != nil {
			m.closeReason = err.Error()
			break
		}

		// 处理包
		msg := msgPack{
			Type:     rawMessage.Type,
			payload:  rawMessage.Payload,
			fullData: data,
		}
		if err = m.dealMsg(msg); err != nil {
			m.closeReason = err.Error()
			break
		}
	}
}

func (m *model) notifyClosed() {
	fullData := message.Must(message.EncodeEventMsg("proxy/closed", message.Args{
		"addr":   m.RemoteAddr().String(),
		"reason": m.closeReason,
	}))

	// 无论m是否订阅closed事件都主动推送
	m.writeChan <- fullData

	m.eventBroadcast <- stateOrEventMessage{
		Name:     "proxy/closed",
		FullData: fullData,
	}
}

func (m *model) writer() {
	defer m.Close()
	for {
		select {
		// 退出
		case <-m.writerQuit:
			// NOTE: 只有主动退出了才return, 其他情况忽略错误继续执行
			// NOTE: 这样能保证通过通道向writer发数据时，不会因为writer退出而死锁！！！
			return
		// 发送数据
		case data := <-m.writeChan:
			// 记录发送数据
			m.log.Println("-->", m.RemoteAddr().String(), string(data))
			_ = m.WriteMsg(data)
		}
	}
}

func (m *model) dealMsg(msg msgPack) error {
	select {
	case <-m.added:
		for len(m.buffer) > 0 {
			if err := m.onMsg(m.buffer[0]); err != nil {
				return err
			}
			m.buffer = m.buffer[1:]
		}
		return m.onMsg(msg)
	default:
		if isTransMsg(msg) {
			m.buffer = append(m.buffer, msg)
			return nil
		}
	}
	return m.onMsg(msg)

}

var trans = map[string]struct{}{
	"set-subscribe-state":    {},
	"add-subscribe-state":    {},
	"remove-subscribe-state": {},
	"clear-subscribe-state":  {},
	"set-subscribe-event":    {},
	"add-subscribe-event":    {},
	"remove-subscribe-event": {},
	"clear-subscribe-event":  {},
	"state":                  {},
	"event":                  {},
	"call":                   {},
	"response":               {},
}

func isTransMsg(msg msgPack) bool {
	_, seen := trans[msg.Type]
	return seen
}

func (m *model) onMsg(msg msgPack) error {
	if handler, seen := m.msgHandlers[msg.Type]; seen {
		return handler(msg)
	}
	return fmt.Errorf("invalid message type %s", msg.Type)
}

func (m *model) onSubState(msg msgPack) error {
	var states []string
	if err := jsoniter.Unmarshal(msg.payload, &states); err != nil {
		return err
	}

	var option int
	switch msg.Type {
	case "set-subscribe-state":
		option = message.SetSub
	case "add-subscribe-state":
		option = message.AddSub
	case "remove-subscribe-state":
		option = message.RemoveSub
	case "clear-subscribe-state":
		option = message.ClearSub
	}

	m.subStateChan <- subStateOrEventMessage{
		Source: m.MetaInfo.Name,
		Type:   option,
		Items:  states,
	}
	return nil
}

func (m *model) onSubEvent(msg msgPack) error {
	var events []string
	if err := jsoniter.Unmarshal(msg.payload, &events); err != nil {
		return err
	}

	var option int

	switch msg.Type {
	case "set-subscribe-event":
		option = message.SetSub
	case "add-subscribe-event":
		option = message.AddSub
	case "remove-subscribe-event":
		option = message.RemoveSub
	case "clear-subscribe-event":
		option = message.ClearSub
	}

	m.subEventChan <- subStateOrEventMessage{
		Source: m.MetaInfo.Name,
		Type:   option,
		Items:  events,
	}
	return nil
}

func (m *model) onState(msg msgPack) error {
	var state message.StatePayload
	if err := jsoniter.Unmarshal(msg.payload, &state); err != nil {
		return err
	}

	// name字段为空或不存在
	if strings.TrimSpace(state.Name) == "" {
		return errors.New("name NOT exist or empty")
	}

	// data字段为null或不存在
	if state.Data == nil {
		return errors.New("data NOT exist or null")
	}

	m.stateBroadcast <- stateOrEventMessage{
		Name:     state.Name,
		FullData: msg.fullData,
	}
	return nil
}

func (m *model) onEvent(msg msgPack) error {
	var event message.EventPayload
	if err := jsoniter.Unmarshal(msg.payload, &event); err != nil {
		return err
	}

	// name字段为空或不存在
	if strings.TrimSpace(event.Name) == "" {
		return errors.New("name NOT exist or empty")
	}

	// args字段为null或不存在
	if event.Args == nil {
		return errors.New("args NOT exist or null")
	}

	m.eventBroadcast <- stateOrEventMessage{
		Name:     event.Name,
		FullData: msg.fullData,
	}
	return nil
}

func (m *model) onCall(msg msgPack) error {
	var call message.CallPayload
	if err := jsoniter.Unmarshal(msg.payload, &call); err != nil {
		return err
	}

	// uuid字段为空或不存在
	if strings.TrimSpace(call.UUID) == "" {
		return errors.New("uuid NOT exist or empty")
	}

	// args字段不存在或为空
	if call.Args == nil {
		errStr := "args NOT exist or empty"
		m.writeChan <- message.Must(message.EncodeRespMsg(call.UUID, errStr, message.Resp{}))
		return nil
	}

	modelName, methodName, err := splitModelName(call.Name)
	if err != nil {
		m.writeChan <- message.Must(message.EncodeRespMsg(call.UUID, err.Error(), message.Resp{}))
		return nil
	}

	m.callChan <- callMessage{
		Source:   m.MetaInfo.Name,
		Model:    modelName,
		Method:   methodName,
		UUID:     call.UUID,
		Args:     call.Args,
		FullData: msg.fullData,
	}
	return nil
}

func (m *model) onResp(msg msgPack) error {
	var resp message.ResponsePayload
	if err := jsoniter.Unmarshal(msg.payload, &resp); err != nil {
		return err
	}

	// uuid字段为空或不存在
	if strings.TrimSpace(resp.UUID) == "" {
		return errors.New("uuid NOT exist or empty")
	}

	m.respChan <- responseMessage{
		Source:   m.MetaInfo.Name,
		UUID:     resp.UUID,
		FullData: msg.fullData,
	}

	return nil
}

func (m *model) onQueryMeta(msgPack) error {
	m.writeChan <- proxyMetaMessage
	return nil
}

func (m *model) onMetaInfo(msg msgPack) error {
	m.onGetMetaOnce.Do(func() {
		m.MetaRaw = msg.payload
		close(m.metaGotChan)
	})

	return nil
}

func splitModelName(fullName string) (string, string, error) {
	index := strings.LastIndex(fullName, "/")
	if index == -1 {
		return "", "", fmt.Errorf("%q missing '/'", fullName)
	}

	if strings.TrimSpace(fullName[:index]) == "" {
		return "", "", fmt.Errorf("no model name in %q", fullName)
	}

	if strings.TrimSpace(fullName[index+1:]) == "" {
		return "", "", fmt.Errorf("no method name in %q", fullName)
	}

	return fullName[:index], fullName[index+1:], nil
}
