package message

import (
	jsoniter "github.com/json-iterator/go"
	"proxy/meta"
)

type Message struct {
	Type    string              `json:"type"`
	Payload jsoniter.RawMessage `json:"payload"`
}

type StateOrEventMessage struct {
	Name     string // 状态或者事件名称
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type CallMessage struct {
	Name     string // 调用的方法全名(模型名/方法名)
	UUID     string // 调用UUID
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
}

type ResponseMessage struct {
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
	Name string              `json:"name"`
	Data jsoniter.RawMessage `json:"data"`
}

type EventPayload struct {
	Name string                         `json:"name"`
	Args map[string]jsoniter.RawMessage `json:"args"`
}

type CallPayload struct {
	Name string                         `json:"name"`
	UUID string                         `json:"uuid"`
	Args map[string]jsoniter.RawMessage `json:"args"`
}

type ResponsePayload struct {
	UUID  string                         `json:"uuid"`
	Error string                         `json:"error"`
	Resp  map[string]jsoniter.RawMessage `json:"resp"`
}
