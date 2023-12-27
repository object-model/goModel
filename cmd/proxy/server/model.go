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

type msg struct {
	msgType  string
	payload  []byte
	fullData []byte
}

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
	bufferQuit      chan struct{}                 // 退出 bufferMsgHandler 的信号
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
	bufferQuitOnce  sync.Once                     // 保证 bufferQuit 只关闭一次
	quitWriterOnce  sync.Once                     // 保证 writerQuit 只关闭一次
	addedOnce       sync.Once                     // 保证 added 只关闭一次
	MetaInfo        *meta.Meta                    // 元信息
	MetaRaw         []byte                        // 原始的元信息
	log             *log.Logger                   // 记录收发数据
	bufferCloseOnce sync.Once                     // 保证buffer仅关闭一次
	buffer          chan msg                      // 挂起的报文
	bufferDone      chan struct{}                 // 挂起报文处理完成信号
	bufferErr       chan struct{}                 // 挂起报文处理出错信号
	bufferExit      chan struct{}                 // bufferMsgHandler 退出信号
	closeReason     string                        // 连接关闭原因
}

func (m *model) Close() error {
	// 保证在未添加的情况下, 退出 bufferMsgHandler
	m.bufferQuitOnce.Do(func() {
		close(m.bufferQuit)
	})
	// 保证在已添加的情况下, 退出 bufferMsgHandler
	m.bufferCloseOnce.Do(func() {
		close(m.buffer)
	})
	return m.RawConn.Close()
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
		// NOTE: 主动关闭，保证 bufferMsgHandler 一定能退出
		m.bufferQuitOnce.Do(func() {
			close(m.bufferQuit)
		})
		m.bufferCloseOnce.Do(func() {
			close(m.buffer)
		})

		// NOTE: 必须等待 bufferMsgHandler 完全退出了
		// NOTE: 否则，会导致提前删除了m, 进一步导致可能出现访问无效内存
		<-m.bufferExit

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
		msg := message.RawMessage{}
		if err = jsoniter.Unmarshal(data, &msg); err != nil {
			m.closeReason = err.Error()
			break
		}

		// 处理包
		if err = m.dealMsg(msg.Type, msg.Payload, data); err != nil {
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

func (m *model) bufferMsgHandler() {
	defer close(m.bufferExit)
	select {
	case <-m.added:
		for msg := range m.buffer {
			err := m.dealTransMsg(msg.msgType, msg.payload, msg.fullData)
			if err != nil {
				close(m.bufferErr)
				return
			}
		}
		close(m.bufferDone)
	case <-m.bufferQuit:
	}
}

func (m *model) dealMsg(msgType string, payload []byte, fullData []byte) error {
	switch msgType {
	// NOTE: 所有需要处理或者转发的报文都需要等待代理完成添加,
	// NOTE: 并等待添加前排队挂起的报文都处理完毕或者出错!
	// NOTE: 目的是严格保证报文的处理顺序!
	case "set-subscribe-state", "add-subscribe-state",
		"remove-subscribe-state", "clear-subscribe-state",
		"set-subscribe-event", "add-subscribe-event",
		"remove-subscribe-event", "clear-subscribe-event",
		"state", "event", "call", "response":
		select {
		case <-m.added:
			m.bufferCloseOnce.Do(func() {
				close(m.buffer)
			})
			select {
			case <-m.bufferDone:
				return m.dealTransMsg(msgType, payload, fullData)
			case <-m.bufferErr:
				return errors.New("buffered message error")
			}
		default:
			select {
			case m.buffer <- msg{
				msgType:  msgType,
				payload:  payload,
				fullData: fullData,
			}:
			default:
				return errors.New("to much cached message")
			}
		}
	case "query-meta":
		return m.onQueryMeta()
	case "meta-info":
		return m.onMetaInfo(payload)
	default:
		return fmt.Errorf("invalid message type %s", msgType)
	}
	return nil
}

func (m *model) dealTransMsg(msgType string, payload []byte, fullData []byte) error {
	switch msgType {
	case "set-subscribe-state", "add-subscribe-state",
		"remove-subscribe-state", "clear-subscribe-state":
		return m.onSubState(msgType, payload)
	case "set-subscribe-event", "add-subscribe-event",
		"remove-subscribe-event", "clear-subscribe-event":
		return m.onSubEvent(msgType, payload)
	case "state":
		return m.onState(payload, fullData)
	case "event":
		return m.onEvent(payload, fullData)
	case "call":
		return m.onCall(payload, fullData)
	case "response":
		return m.onResp(payload, fullData)
	}
	return nil
}

func (m *model) onSubState(Type string, payload []byte) error {
	var states []string
	if err := jsoniter.Unmarshal(payload, &states); err != nil {
		return err
	}

	var option int
	switch Type {
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

func (m *model) onSubEvent(Type string, payload []byte) error {
	var events []string
	if err := jsoniter.Unmarshal(payload, &events); err != nil {
		return err
	}

	var option int

	switch Type {
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

func (m *model) onState(payload []byte, fullData []byte) error {
	var state message.StatePayload
	if err := jsoniter.Unmarshal(payload, &state); err != nil {
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
		FullData: fullData,
	}
	return nil
}

func (m *model) onEvent(payload []byte, fullData []byte) error {
	var event message.EventPayload
	if err := jsoniter.Unmarshal(payload, &event); err != nil {
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
		FullData: fullData,
	}
	return nil
}

func (m *model) onCall(payload []byte, fullData []byte) error {
	var call message.CallPayload
	if err := jsoniter.Unmarshal(payload, &call); err != nil {
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
		FullData: fullData,
	}
	return nil
}

func (m *model) onResp(payload []byte, fullData []byte) error {
	var resp message.ResponsePayload
	if err := jsoniter.Unmarshal(payload, &resp); err != nil {
		return err
	}

	// uuid字段为空或不存在
	if strings.TrimSpace(resp.UUID) == "" {
		return errors.New("uuid NOT exist or empty")
	}

	m.respChan <- responseMessage{
		Source:   m.MetaInfo.Name,
		UUID:     resp.UUID,
		FullData: fullData,
	}

	return nil
}

func (m *model) onQueryMeta() error {
	m.writeChan <- proxyMetaMessage
	return nil
}

func (m *model) onMetaInfo(payload []byte) error {
	m.onGetMetaOnce.Do(func() {
		m.MetaRaw = payload
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
