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
	Source   string // 发送者的物模型名称
	Name     string // 状态或者事件名称
	FullData []byte // 全报文原始数据，是Message类型序列化的结果
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
	UUID  string                 `json:"uuid"`
	Error string                 `json:"error"`
	Resp  map[string]interface{} `json:"resp"`
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
