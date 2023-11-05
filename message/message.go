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
	Type    string              `json:"type"`
	Payload jsoniter.RawMessage `json:"payload"`
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
	Source   string // 调用者的模型名
	Model    string // 调用目标的模型名
	UUID     string // 调用UUID
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type ResponseMessage struct {
	Source   string // 发送响应报文的模型名
	UUID     string // 调用UUID
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type MetaMessage struct {
	meta.Meta
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type SubStatePayload []string

type SubEventPayload []string

type StatePayload struct {
	Name string      `json:"name"`
	Data interface{} `json:"data"`
}

type EventPayload struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type CallPayload struct {
	Name string                 `json:"name"`
	UUID string                 `json:"uuid"`
	Args map[string]interface{} `json:"args"`
}

type ResponsePayload struct {
	UUID     string                 `json:"uuid"`
	Error    string                 `json:"error"`
	Response map[string]interface{} `json:"response"`
}

func NewResponseFullData(UUID string, Error string, Resp map[string]interface{}) []byte {
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
