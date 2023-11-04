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

type ModelConnection struct {
	net.Conn                                              // 原始连接
	quit               chan struct{}                      // 退出writer的信号
	localSubStateCh    chan updateSubTableMsg             // 更新本地状态订阅通道
	localSubEventCh    chan updateSubTableMsg             // 更新本地事件订阅通道
	remoteSubStateCh   chan updateSubTableMsg             // 更新远端状态订阅通道, 会触发订阅状态报文发送
	remoteSubEventCh   chan updateSubTableMsg             // 更新远端事件订阅通道, 会触发订阅事件报文发送
	querySubState      chan chan []string                 // 查询状态订阅通道
	querySubEvent      chan chan []string                 // 查询事件订阅通道
	removeConnCh       chan<- *ModelConnection            // 删除连接通道
	stateBroadcast     chan<- message.StateOrEventMessage // 状态广播通道
	eventBroadcast     chan<- message.StateOrEventMessage // 事件广播通道
	callChan           chan<- message.CallMessage         // 调用请求通道
	respChan           chan<- message.ResponseMessage     // 响应结果通道
	stateWriteChan     chan message.StateOrEventMessage   // 状态写入通道
	eventWriteChan     chan message.StateOrEventMessage   // 事件写入通道
	callWriteChan      chan message.CallMessage           // 调用写入通道
	respWriteChan      chan message.ResponseMessage       // 响应写入通道
	queryProxyMetaChan chan struct{}                      // 查询代理物模型消息通道
	queryMetaChan      chan struct{}                      // 查询元信息通道
	metaGotChan        chan struct{}                      // 收到元信息消息通道
	queryOnce          sync.Once                          // 保证只查询一次元信息
	onGetMetaOnce      sync.Once                          // 保证只响应一次元信息结果报文
	quitOnce           sync.Once                          // 保证只退出一次
	MetaInfo           message.MetaMessage                // 元信息

}

func (c *ModelConnection) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	length := uint32(len(data))
	err := binary.Write(c.Conn, binary.LittleEndian, &length)
	if err != nil {
		return 0, err
	}

	return c.Conn.Write(data)
}

func (c *ModelConnection) GetSubStates() []string {
	queryChan := make(chan []string, 1)
	c.querySubState <- queryChan
	return <-queryChan
}

func (c *ModelConnection) SetSubStates(states []string) {
	c.localSubStateCh <- updateSubTableMsg{
		option: setSub,
		items:  states,
	}
}

func (c *ModelConnection) AddSubStates(states []string) {
	c.localSubStateCh <- updateSubTableMsg{
		option: addSub,
		items:  states,
	}
}

func (c *ModelConnection) RemoveSubStates(states []string) {
	c.localSubStateCh <- updateSubTableMsg{
		option: removeSub,
		items:  states,
	}
}

func (c *ModelConnection) ClearSubStates() {
	c.localSubStateCh <- updateSubTableMsg{
		option: clearSub,
	}
}

func (c *ModelConnection) GetSubEvents() []string {
	queryChan := make(chan []string, 1)
	c.querySubEvent <- queryChan
	return <-queryChan
}

func (c *ModelConnection) SetSubEvents(events []string) {
	c.localSubEventCh <- updateSubTableMsg{
		option: setSub,
		items:  events,
	}
}

func (c *ModelConnection) AddSubEvents(events []string) {
	c.localSubEventCh <- updateSubTableMsg{
		option: addSub,
		items:  events,
	}
}

func (c *ModelConnection) RemoveSubEvents(events []string) {
	c.localSubEventCh <- updateSubTableMsg{
		option: removeSub,
		items:  events,
	}
}

func (c *ModelConnection) ClearSubEvents() {
	c.localSubEventCh <- updateSubTableMsg{
		option: clearSub,
	}
}

func (c *ModelConnection) quitWriter() {
	c.quitOnce.Do(func() {
		close(c.quit)
	})
}

func (c *ModelConnection) queryMeta(timeout time.Duration) error {
	c.queryOnce.Do(func() {
		c.queryMetaChan <- struct{}{}
	})

	select {
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	case <-c.metaGotChan:
		break
	}

	return nil
}

func (c *ModelConnection) reader() {
	defer func() {
		c.removeConnCh <- c
	}()
	for {
		// 读取报文
		data, err := c.readMsg()
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
		if err = c.dealMsg(msg.Type, msg.Payload, data); err != nil {
			break
		}
	}
}

func (c *ModelConnection) writer() {
	defer c.Close()
	wantedStates := make(map[string]int) // 链路期望接收的状态列表
	wantedEvents := make(map[string]int) // 链路期望接收的事件列表
	for {
		select {
		// 退出
		case <-c.quit:
			// NOTE: 只有主动退出了才return, 其他情况忽略错误继续执行
			// NOTE: 这样能保证通过通道向writer发数据时，不会因为writer退出而死锁！！！
			return

		// 查询连接对应的物模型元信息
		case <-c.queryMetaChan:
			_, _ = c.Write([]byte(queryMetaJSON))

		// 查询代理对应的物模型元信息
		case <-c.queryProxyMetaChan:
			_, _ = c.Write([]byte(proxyMetaJSON))

		// 状态发布
		case state := <-c.stateWriteChan:
			c.pubStateOrEvent(wantedStates, state)

		// 事件发布
		case event := <-c.eventWriteChan:
			c.pubStateOrEvent(wantedEvents, event)

		// 调用消息
		case call := <-c.callWriteChan:
			_, _ = c.Write(call.FullData)

		// 调用响应
		case resp := <-c.respWriteChan:
			_, _ = c.Write(resp.FullData)

		// 更新连接本地的订阅状态列表
		case stateReq := <-c.localSubStateCh:
			wantedStates = dealSubReq(stateReq, wantedStates)

		// 更新连接本地的订阅事件列表
		case eventReq := <-c.localSubEventCh:
			wantedEvents = dealSubReq(eventReq, wantedEvents)

		// 发送订阅状态报文
		case stateReq := <-c.remoteSubStateCh:
			c.sendSubStateMsg(stateReq)

		// 发送订阅事件报文
		case eventReq := <-c.remoteSubEventCh:
			c.sendSubEventMsg(eventReq)

		// 查询连接订阅状态列表
		case resChan := <-c.querySubState:
			res := make([]string, 0, len(wantedStates))
			for state := range wantedStates {
				res = append(res, state)
			}
			resChan <- res

		// 查询连接订阅事件列表
		case resChan := <-c.querySubEvent:
			res := make([]string, 0, len(wantedEvents))
			for event := range wantedEvents {
				res = append(res, event)
			}
			resChan <- res
		}
	}
}

func (c *ModelConnection) readMsg() ([]byte, error) {
	// 读取长度
	var length uint32
	err := binary.Read(c, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}

	// 读取数据
	data := make([]byte, length)
	if err = binary.Read(c, binary.LittleEndian, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *ModelConnection) pubStateOrEvent(pubSet map[string]int, state message.StateOrEventMessage) {
	if _, seen := pubSet[state.Name]; !seen {
		return
	}
	_, _ = c.Write(state.FullData)
	return
}

func (c *ModelConnection) dealMsg(msgType string, payload []byte, fullData []byte) error {
	switch msgType {
	case "set-subscribe-state", "add-subscribe-state",
		"remove-subscribe-state", "clear-subscribe-state":
		return c.onSubState(msgType, payload)
	case "set-subscribe-event", "add-subscribe-event",
		"remove-subscribe-event", "clear-subscribe-event":
		return c.onSubEvent(msgType, payload)
	case "state":
		return c.onState(payload, fullData)
	case "event":
		return c.onEvent(payload, fullData)
	case "call":
		return c.onCall(payload, fullData)
	case "response":
		return c.onResp(payload, fullData)
	case "query-meta":
		return c.onQueryMeta()
	case "meta-info":
		return c.onMetaInfo(payload, fullData)
	default:
		return fmt.Errorf("invalid message type %s", msgType)
	}

	return nil
}

func (c *ModelConnection) onSubState(Type string, payload []byte) error {
	var states []string
	if err := jsoniter.Unmarshal(payload, &states); err != nil {
		return err
	}

	switch Type {
	case "set-subscribe-state":
		c.SetSubStates(states)
	case "add-subscribe-state":
		c.AddSubStates(states)
	case "remove-subscribe-state":
		c.RemoveSubStates(states)
	case "clear-subscribe-state":
		c.ClearSubStates()
	}

	return nil
}

func (c *ModelConnection) onSubEvent(Type string, payload []byte) error {
	var events []string
	if err := jsoniter.Unmarshal(payload, &events); err != nil {
		return err
	}

	switch Type {
	case "set-subscribe-event":
		c.SetSubEvents(events)
	case "add-subscribe-":
		c.AddSubEvents(events)
	case "remove-subscribe-":
		c.RemoveSubEvents(events)
	case "clear-subscribe-event":
		c.ClearSubEvents()
	}

	return nil
}

func (c *ModelConnection) onState(payload []byte, fullData []byte) error {
	var state message.StatePayload
	if err := jsoniter.Unmarshal(payload, &state); err != nil {
		return err
	}

	c.stateBroadcast <- message.StateOrEventMessage{
		Source:   c.MetaInfo.Name,
		Name:     state.Name,
		FullData: fullData,
	}

	return nil
}

func (c *ModelConnection) onEvent(payload []byte, fullData []byte) error {
	var event message.EventPayload
	if err := jsoniter.Unmarshal(payload, &event); err != nil {
		return err
	}

	c.eventBroadcast <- message.StateOrEventMessage{
		Source:   c.MetaInfo.Name,
		Name:     event.Name,
		FullData: fullData,
	}

	return nil
}

func (c *ModelConnection) onCall(payload []byte, fullData []byte) error {
	var call message.CallPayload
	if err := jsoniter.Unmarshal(payload, &call); err != nil {
		return err
	}

	modelName, err := splitModelName(call.Name)
	if err != nil {
		resp := make(map[string]interface{})
		c.respWriteChan <- message.ResponseMessage{
			Source:   "proxy",
			UUID:     call.UUID,
			FullData: message.NewResponseFullData(call.UUID, err.Error(), resp),
		}
		return nil
	}

	c.callChan <- message.CallMessage{
		Source:   c.MetaInfo.Name,
		Model:    modelName,
		UUID:     call.UUID,
		FullData: fullData,
	}

	return nil
}

func (c *ModelConnection) onResp(payload []byte, fullData []byte) error {
	var resp message.ResponsePayload
	if err := jsoniter.Unmarshal(payload, &resp); err != nil {
		return err
	}

	c.respChan <- message.ResponseMessage{
		Source:   c.MetaInfo.Name,
		UUID:     resp.UUID,
		FullData: fullData,
	}

	return nil
}

func (c *ModelConnection) onQueryMeta() error {
	c.queryProxyMetaChan <- struct{}{}
	return nil
}

func (c *ModelConnection) onMetaInfo(payload []byte, fullData []byte) error {
	var metaInfo meta.Meta
	if err := jsoniter.Unmarshal(payload, &metaInfo); err != nil {
		return err
	}

	c.onGetMetaOnce.Do(func() {
		c.MetaInfo = message.MetaMessage{
			Meta:     metaInfo,
			FullData: fullData,
		}

		close(c.metaGotChan)
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

func (c *ModelConnection) sendSubStateMsg(req updateSubTableMsg) {
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

	_, _ = c.Write(data)
}

func (c *ModelConnection) sendSubEventMsg(req updateSubTableMsg) {
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

	_, _ = c.Write(data)
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
