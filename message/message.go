package message

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"proxy/meta"
)

const (
	SetSub    = iota // 设置订阅
	AddSub           // 添加订阅
	RemoveSub        // 删除订阅
	ClearSub         // 清空订阅
)

type Message struct {
	Type    string              `json:"type"`    // 报文类型
	Payload jsoniter.RawMessage `json:"payload"` // 报文内容
}

type StateOrEventMessage struct {
	Source   string // 发送者的物模型名称
	Name     string // 状态或者事件名称
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type SubStateOrEventMessage struct {
	Source string   // 发送者的物模型名称
	Type   int      // 订阅类型
	Items  []string // 状态或者事件列表
}

type CallMessage struct {
	Source   string                         // 调用者的模型名
	Model    string                         // 调用目标的模型名
	Method   string                         // 调用目标的方法名
	UUID     string                         // 调用UUID
	Args     map[string]jsoniter.RawMessage // 调用参数
	FullData []byte                         // 全报文原始数据，是Message类型序列化的结果
}

type ResponseMessage struct {
	Source   string // 发送响应报文的模型名
	UUID     string // 调用UUID
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type MetaMessage struct {
	meta.Meta
	RawData []byte // 元信息JSON串原始数据
}

type SubStatePayload []string

type SubEventPayload []string

type StatePayload struct {
	Name string      `json:"name"` // 状态全名: 模型名/状态名
	Data interface{} `json:"data"` // 状态数据
}

type EventPayload struct {
	Name string                 `json:"name"` // 事件全名: 模型名/事件名
	Args map[string]interface{} `json:"args"` // 事件参数
}

type CallPayload struct {
	Name string                         `json:"name"` // 调用的全方法名: 模型名/方法名
	UUID string                         `json:"uuid"` // 调用的UUID
	Args map[string]jsoniter.RawMessage `json:"args"` // 调用的参数
}

type ResponsePayload struct {
	UUID     string                 `json:"uuid"`     // 响应的UUID
	Error    string                 `json:"error"`    // 错误字符串
	Response map[string]interface{} `json:"response"` // 响应结果
}

type Resp map[string]interface{}

func NewResponseFullData(UUID string, Error string, Resp Resp) []byte {
	msg := map[string]interface{}{
		"type": "response",
		"payload": ResponsePayload{
			UUID,
			Error,
			Resp,
		},
	}

	data, err := jsoniter.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return data
}

func NewPubStateMessage(Type int, items []string) ([]byte, error) {
	var typeStr string
	switch Type {
	case SetSub:
		typeStr = "set-subscribe-state"
	case AddSub:
		typeStr = "add-subscribe-state"
	case RemoveSub:
		typeStr = "remove-subscribe-state"
	case ClearSub:
		typeStr = "clear-subscribe-state"
	default:
		return nil, fmt.Errorf("invalid Type")
	}

	msg := map[string]interface{}{
		"type":    typeStr,
		"payload": items,
	}

	return jsoniter.Marshal(msg)
}

func NewPubEventMessage(Type int, items []string) ([]byte, error) {
	var typeStr string
	switch Type {
	case SetSub:
		typeStr = "set-subscribe-event"
	case AddSub:
		typeStr = "add-subscribe-event"
	case RemoveSub:
		typeStr = "remove-subscribe-event"
	case ClearSub:
		typeStr = "clear-subscribe-event"
	default:
		return nil, fmt.Errorf("invalid Type")
	}

	msg := map[string]interface{}{
		"type":    typeStr,
		"payload": items,
	}

	return jsoniter.Marshal(msg)
}

func UpdatePubTable(req SubStateOrEventMessage, pubSet map[string]struct{}) map[string]struct{} {
	switch req.Type {
	case SetSub:
		pubSet = make(map[string]struct{})
		for _, sub := range req.Items {
			pubSet[sub] = struct{}{}
		}
	case AddSub:
		for _, sub := range req.Items {
			pubSet[sub] = struct{}{}
		}
	case RemoveSub:
		for _, sub := range req.Items {
			delete(pubSet, sub)
		}
	case ClearSub:
		pubSet = make(map[string]struct{})
	}

	return pubSet
}
