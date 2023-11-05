package server

import (
	"encoding/binary"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"net"
	"proxy/message"
	"proxy/meta"
	"strings"
	"sync"
	"time"
)

const (
	setSub    = iota // 设置订阅
	addSub           // 添加订阅
	removeSub        // 删除订阅
	clearSub         // 清空订阅
)

type updateSubTableMsg struct {
	option int      // 更新动作类型
	items  []string // 状态和事件列表
}

const queryMetaJSON = `
{
	"type": "query-meta",
	"payload": null
}
`

const proxyMetaJSON = `
{
	"type": "meta-info",
	"payload": {
		"name": "proxy",
		"description": "model proxy service",
		"version": "1.0.0",
		"state": [],
		"event": [],
		"method": []
	}
}
`

type model struct {
	net.Conn                                            // 原始连接
	quit             chan struct{}                      // 退出writer的信号
	localSubStateCh  chan updateSubTableMsg             // 更新本地状态订阅通道
	localSubEventCh  chan updateSubTableMsg             // 更新本地事件订阅通道
	remoteSubStateCh chan updateSubTableMsg             // 更新远端状态订阅通道, 会触发订阅状态报文发送
	remoteSubEventCh chan updateSubTableMsg             // 更新远端事件订阅通道, 会触发订阅事件报文发送
	querySubState    chan chan []string                 // 查询状态订阅通道
	querySubEvent    chan chan []string                 // 查询事件订阅通道
	removeConnCh     chan<- *model                      // 删除连接通道
	stateBroadcast   chan<- message.StateOrEventMessage // 状态广播通道
	eventBroadcast   chan<- message.StateOrEventMessage // 事件广播通道
	callChan         chan<- message.CallMessage         // 调用请求通道
	respChan         chan<- message.ResponseMessage     // 响应结果通道
	serverDone       <-chan struct{}                    // Server 完成退出信息
	stateWriteChan   chan message.StateOrEventMessage   // 状态写入通道
	eventWriteChan   chan message.StateOrEventMessage   // 事件写入通道
	callWriteChan    chan message.CallMessage           // 调用写入通道
	respWriteChan    chan message.ResponseMessage       // 响应写入通道
	writeChan        chan []byte                        // 数据写入通道
	metaGotChan      chan struct{}                      // 收到元信息消息通道
	queryOnce        sync.Once                          // 保证只查询一次元信息
	onGetMetaOnce    sync.Once                          // 保证只响应一次元信息结果报文
	quitOnce         sync.Once                          // 保证只退出一次
	MetaInfo         message.MetaMessage                // 元信息

}

func (m *model) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	length := uint32(len(data))
	err := binary.Write(m.Conn, binary.LittleEndian, &length)
	if err != nil {
		return 0, err
	}

	return m.Conn.Write(data)
}

func (m *model) GetSubStates() []string {
	queryChan := make(chan []string, 1)
	m.querySubState <- queryChan
	return <-queryChan
}

func (m *model) SetSubStates(states []string) {
	m.localSubStateCh <- updateSubTableMsg{
		option: setSub,
		items:  states,
	}
}

func (m *model) AddSubStates(states []string) {
	m.localSubStateCh <- updateSubTableMsg{
		option: addSub,
		items:  states,
	}
}

func (m *model) RemoveSubStates(states []string) {
	m.localSubStateCh <- updateSubTableMsg{
		option: removeSub,
		items:  states,
	}
}

func (m *model) ClearSubStates() {
	m.localSubStateCh <- updateSubTableMsg{
		option: clearSub,
	}
}

func (m *model) GetSubEvents() []string {
	queryChan := make(chan []string, 1)
	m.querySubEvent <- queryChan
	return <-queryChan
}

func (m *model) SetSubEvents(events []string) {
	m.localSubEventCh <- updateSubTableMsg{
		option: setSub,
		items:  events,
	}
}

func (m *model) AddSubEvents(events []string) {
	m.localSubEventCh <- updateSubTableMsg{
		option: addSub,
		items:  events,
	}
}

func (m *model) RemoveSubEvents(events []string) {
	m.localSubEventCh <- updateSubTableMsg{
		option: removeSub,
		items:  events,
	}
}

func (m *model) ClearSubEvents() {
	m.localSubEventCh <- updateSubTableMsg{
		option: clearSub,
	}
}

func (m *model) quitWriter() {
	m.quitOnce.Do(func() {
		close(m.quit)
	})
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
		select {
		case m.removeConnCh <- m: // 在Server未退出的情况下，通过Server退出writer
		case <-m.serverDone:
			// NOTE: 在Server完全退出的情况下，可自行退出writer
			// NOTE: 否则会导致提前退出了writer的情况下，run还在向writer发送信号，
			// NOTE: 从而导致run无法退出
			m.quitWriter()
		}
	}()
	for {
		// 读取报文
		data, err := m.readMsg()
		if err != nil {
			break
		}

		if len(data) <= 0 {
			continue
		}

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
	wantedStates := make(map[string]int) // 链路期望接收的状态列表
	wantedEvents := make(map[string]int) // 链路期望接收的事件列表
	for {
		select {
		// 退出
		case <-m.quit:
			// NOTE: 只有主动退出了才return, 其他情况忽略错误继续执行
			// NOTE: 这样能保证通过通道向writer发数据时，不会因为writer退出而死锁！！！
			return

		// 状态发布
		case state := <-m.stateWriteChan:
			m.pubStateOrEvent(wantedStates, state)

		// 事件发布
		case event := <-m.eventWriteChan:
			m.pubStateOrEvent(wantedEvents, event)

		// 调用消息
		case call := <-m.callWriteChan:
			_, _ = m.Write(call.FullData)

		// 调用响应
		case resp := <-m.respWriteChan:
			_, _ = m.Write(resp.FullData)

		// 发送数据
		case data := <-m.writeChan:
			_, _ = m.Write(data)

		// 更新连接本地的订阅状态列表
		case stateReq := <-m.localSubStateCh:
			wantedStates = dealSubReq(stateReq, wantedStates)

		// 更新连接本地的订阅事件列表
		case eventReq := <-m.localSubEventCh:
			wantedEvents = dealSubReq(eventReq, wantedEvents)

		// 发送订阅状态报文
		case stateReq := <-m.remoteSubStateCh:
			m.sendSubStateMsg(stateReq)

		// 发送订阅事件报文
		case eventReq := <-m.remoteSubEventCh:
			m.sendSubEventMsg(eventReq)

		// 查询连接订阅状态列表
		case resChan := <-m.querySubState:
			res := make([]string, 0, len(wantedStates))
			for state := range wantedStates {
				res = append(res, state)
			}
			resChan <- res

		// 查询连接订阅事件列表
		case resChan := <-m.querySubEvent:
			res := make([]string, 0, len(wantedEvents))
			for event := range wantedEvents {
				res = append(res, event)
			}
			resChan <- res
		}
	}
}

func (m *model) readMsg() ([]byte, error) {
	// 读取长度
	var length uint32
	err := binary.Read(m, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}

	// 读取数据
	data := make([]byte, length)
	if err = binary.Read(m, binary.LittleEndian, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *model) pubStateOrEvent(pubSet map[string]int, state message.StateOrEventMessage) {
	if _, seen := pubSet[state.Name]; !seen {
		return
	}
	_, _ = m.Write(state.FullData)
	return
}

func (m *model) dealMsg(msgType string, payload []byte, fullData []byte) error {
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
	case "query-meta":
		return m.onQueryMeta()
	case "meta-info":
		return m.onMetaInfo(payload, fullData)
	default:
		return fmt.Errorf("invalid message type %s", msgType)
	}

	return nil
}

func (m *model) onSubState(Type string, payload []byte) error {
	var states []string
	if err := jsoniter.Unmarshal(payload, &states); err != nil {
		return err
	}

	switch Type {
	case "set-subscribe-state":
		m.SetSubStates(states)
	case "add-subscribe-state":
		m.AddSubStates(states)
	case "remove-subscribe-state":
		m.RemoveSubStates(states)
	case "clear-subscribe-state":
		m.ClearSubStates()
	}

	return nil
}

func (m *model) onSubEvent(Type string, payload []byte) error {
	var events []string
	if err := jsoniter.Unmarshal(payload, &events); err != nil {
		return err
	}

	switch Type {
	case "set-subscribe-event":
		m.SetSubEvents(events)
	case "add-subscribe-":
		m.AddSubEvents(events)
	case "remove-subscribe-":
		m.RemoveSubEvents(events)
	case "clear-subscribe-event":
		m.ClearSubEvents()
	}

	return nil
}

func (m *model) onState(payload []byte, fullData []byte) error {
	var state message.StatePayload
	if err := jsoniter.Unmarshal(payload, &state); err != nil {
		return err
	}

	select {
	case m.stateBroadcast <- message.StateOrEventMessage{
		Source:   m.MetaInfo.Name,
		Name:     state.Name,
		FullData: fullData,
	}:
	case <-m.serverDone:
		return fmt.Errorf("proxy have quit")
	}

	return nil
}

func (m *model) onEvent(payload []byte, fullData []byte) error {
	var event message.EventPayload
	if err := jsoniter.Unmarshal(payload, &event); err != nil {
		return err
	}

	select {
	case m.eventBroadcast <- message.StateOrEventMessage{
		Source:   m.MetaInfo.Name,
		Name:     event.Name,
		FullData: fullData,
	}:
	case <-m.serverDone:
		return fmt.Errorf("proxy have quit")
	}

	return nil
}

func (m *model) onCall(payload []byte, fullData []byte) error {
	var call message.CallPayload
	if err := jsoniter.Unmarshal(payload, &call); err != nil {
		return err
	}

	modelName, err := splitModelName(call.Name)
	if err != nil {
		resp := make(map[string]interface{})
		m.writeChan <- message.NewResponseFullData(call.UUID, err.Error(), resp)
		return nil
	}

	select {
	case m.callChan <- message.CallMessage{
		Source:   m.MetaInfo.Name,
		Model:    modelName,
		UUID:     call.UUID,
		FullData: fullData,
	}:
	case <-m.serverDone:
		resp := make(map[string]interface{})
		m.respWriteChan <- message.ResponseMessage{
			Source:   "proxy",
			UUID:     call.UUID,
			FullData: message.NewResponseFullData(call.UUID, "proxy have quit", resp),
		}
		return fmt.Errorf("proxy have quit")
	}

	return nil
}

func (m *model) onResp(payload []byte, fullData []byte) error {
	var resp message.ResponsePayload
	if err := jsoniter.Unmarshal(payload, &resp); err != nil {
		return err
	}

	select {
	case m.respChan <- message.ResponseMessage{
		Source:   m.MetaInfo.Name,
		UUID:     resp.UUID,
		FullData: fullData,
	}:
	case <-m.serverDone:
		return fmt.Errorf("proxy have quit")
	}

	return nil
}

func (m *model) onQueryMeta() error {
	m.writeChan <- []byte(proxyMetaJSON)
	return nil
}

func (m *model) onMetaInfo(payload []byte, fullData []byte) error {
	var metaInfo meta.Meta
	if err := jsoniter.Unmarshal(payload, &metaInfo); err != nil {
		return err
	}

	m.onGetMetaOnce.Do(func() {
		m.MetaInfo = message.MetaMessage{
			Meta:     metaInfo,
			FullData: fullData,
		}

		close(m.metaGotChan)
	})

	return nil
}

func dealSubReq(req updateSubTableMsg, pubSet map[string]int) map[string]int {
	switch req.option {
	case setSub:
		pubSet = make(map[string]int)
		for _, sub := range req.items {
			pubSet[sub] = 0
		}
	case addSub:
		for _, sub := range req.items {
			pubSet[sub] = 0
		}
	case removeSub:
		for _, sub := range req.items {
			delete(pubSet, sub)
		}
	case clearSub:
		pubSet = make(map[string]int)
	}

	return pubSet
}

func (m *model) sendSubStateMsg(req updateSubTableMsg) {
	msg := message.Message{}
	switch req.option {
	case setSub:
		msg.Type = "set-subscribe-state"
	case addSub:
		msg.Type = "add-subscribe-state"
	case removeSub:
		msg.Type = "remove-subscribe-state"
	case clearSub:
		msg.Type = "clear-subscribe-state"
	default:
		return
	}

	msg.Payload, _ = jsoniter.Marshal(req.items)
	data, _ := jsoniter.Marshal(msg)

	_, _ = m.Write(data)
}

func (m *model) sendSubEventMsg(req updateSubTableMsg) {
	msg := message.Message{}
	switch req.option {
	case setSub:
		msg.Type = "set-subscribe-event"
	case addSub:
		msg.Type = "add-subscribe-event"
	case removeSub:
		msg.Type = "remove-subscribe-event"
	case clearSub:
		msg.Type = "clear-subscribe-event"
	default:
		return
	}

	msg.Payload, _ = jsoniter.Marshal(req.items)
	data, _ := jsoniter.Marshal(msg)

	_, _ = m.Write(data)
}

func splitModelName(fullName string) (string, error) {
	index := strings.LastIndex(fullName, "/")
	if index == -1 {
		return "", fmt.Errorf("%q missing '/'", fullName)
	}

	if strings.Trim(fullName[:index], " \t\n\r\f\v") == "" {
		return "", fmt.Errorf("no model name in %q", fullName)
	}

	if strings.Trim(fullName[index+1:], " \t\n\n\f\v") == "" {
		return "", fmt.Errorf("no method name in %q", fullName)
	}

	return fullName[:index], nil
}
