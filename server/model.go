package server

import (
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"log"
	"net"
	"proxy/message"
	"proxy/meta"
	"strings"
	"sync"
	"time"
)

var queryMetaJSON = strings.Join(strings.Fields(`
{
	"type": "query-meta",
	"payload": null
}`), "")

type ModelConn interface {
	Close() error
	RemoteAddr() net.Addr

	// ReadMsg 从物模型连接中读取完整的一包物模型报文并返回读取的报文和错误信息
	ReadMsg() ([]byte, error)

	// WriteMsg 将物模型报文msg通过连接发送到网络上
	WriteMsg(msg []byte) error
}

type msg struct {
	msgType  string
	payload  []byte
	fullData []byte
}

type model struct {
	ModelConn                                            // 原始连接
	readerQuit     chan struct{}                         // 退出 reader 的信号
	writerQuit     chan struct{}                         // 退出 writer 的信号
	added          chan struct{}                         // 连接已经加入 Server 信号
	removeConnCh   chan<- *model                         // 删除连接通道
	stateBroadcast chan<- message.StateOrEventMessage    // 状态广播通道
	eventBroadcast chan<- message.StateOrEventMessage    // 事件广播通道
	callChan       chan<- message.CallMessage            // 调用请求通道
	respChan       chan<- message.ResponseMessage        // 响应结果通道
	subStateChan   chan<- message.SubStateOrEventMessage // 更新状态订阅写入通道
	subEventChan   chan<- message.SubStateOrEventMessage // 更新事件订阅写入通道
	writeChan      chan []byte                           // 数据写入通道
	metaGotChan    chan struct{}                         // 收到元信息消息通道
	queryOnce      sync.Once                             // 保证只查询一次元信息
	onGetMetaOnce  sync.Once                             // 保证只响应一次元信息结果报文
	quitReaderOnce sync.Once                             // 保证 readerQuit 只关闭一次
	quitWriterOnce sync.Once                             // 保证 writerQuit 只关闭一次
	addedOnce      sync.Once                             // 保证 added 只关闭一次
	MetaInfo       message.MetaMessage                   // 元信息
	log            *log.Logger                           // 记录收发数据
	buffer         chan msg                              // 挂起的报文
	bufferDone     chan error                            // 挂起报文处理完成信号
}

func (m *model) Close() error {
	m.quitReaderOnce.Do(func() {
		close(m.readerQuit)
	})
	return m.ModelConn.Close()
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
		m.writeChan <- []byte(queryMetaJSON)
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
		m.removeConnCh <- m // 通过Server退出writer
	}()
	for {
		// 读取报文
		data, err := m.ReadMsg()
		if err != nil {
			break
		}

		if len(data) <= 0 {
			continue
		}

		// 记录接收数据
		m.log.Println("<--", m.RemoteAddr().String(), string(data))

		// 解析JSON报文
		msg := message.Message{}
		if err = jsoniter.Unmarshal(data, &msg); err != nil {
			break
		}

		// 处理包
		if err = m.dealMsg(msg.Type, msg.Payload, data); err != nil {
			break
		}
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
	var err error = nil
	defer func() {
		m.bufferDone <- err
	}()
	select {
	case <-m.added:
		for msg := range m.buffer {
			err = m.dealTransMsg(msg.msgType, msg.payload, msg.fullData)
			if err != nil {
				break
			}
		}
	case <-m.readerQuit:
		return
	}
}

func (m *model) dealMsg(msgType string, payload []byte, fullData []byte) error {
	switch msgType {
	case "set-subscribe-state", "add-subscribe-state",
		"remove-subscribe-state", "clear-subscribe-state",
		"set-subscribe-event", "add-subscribe-event",
		"remove-subscribe-event", "clear-subscribe-event",
		"state", "event", "call", "response":
		select {
		case <-m.added:
			close(m.buffer)
			if err := <-m.bufferDone; err != nil {
				return err
			}
			return m.dealTransMsg(msgType, payload, fullData)
		default:
			select {
			case m.buffer <- msg{
				msgType:  msgType,
				payload:  payload,
				fullData: fullData,
			}:
			default:
				return errors.New("to much message cached")
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

	m.subStateChan <- message.SubStateOrEventMessage{
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

	m.subEventChan <- message.SubStateOrEventMessage{
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

	m.stateBroadcast <- message.StateOrEventMessage{
		Source:   m.MetaInfo.Name,
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

	m.eventBroadcast <- message.StateOrEventMessage{
		Source:   m.MetaInfo.Name,
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

	modelName, methodName, err := splitModelName(call.Name)
	if err != nil {
		resp := make(map[string]interface{})
		m.writeChan <- message.NewResponseFullData(call.UUID, err.Error(), resp)
		return nil
	}

	m.callChan <- message.CallMessage{
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

	m.respChan <- message.ResponseMessage{
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
	var metaInfo meta.Meta

	// 解析
	if err := jsoniter.Unmarshal(payload, &metaInfo); err != nil {
		return err
	}

	m.onGetMetaOnce.Do(func() {
		m.MetaInfo = message.MetaMessage{
			Meta:    metaInfo,
			RawData: payload,
		}
		close(m.metaGotChan)
	})

	return nil
}

func splitModelName(fullName string) (string, string, error) {
	index := strings.LastIndex(fullName, "/")
	if index == -1 {
		return "", "", fmt.Errorf("%q missing '/'", fullName)
	}

	if strings.Trim(fullName[:index], " \t\n\r\f\v") == "" {
		return "", "", fmt.Errorf("no model name in %q", fullName)
	}

	if strings.Trim(fullName[index+1:], " \t\n\n\f\v") == "" {
		return "", "", fmt.Errorf("no method name in %q", fullName)
	}

	return fullName[:index], fullName[index+1:], nil
}
