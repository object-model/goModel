package message

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
)

const (
	SetSub    = iota // 设置订阅
	AddSub           // 添加订阅
	RemoveSub        // 删除订阅
	ClearSub         // 清空订阅
)

// 物模型报文定义
type RawMessage struct {
	Type    string              `json:"type"`    // 报文类型
	Payload jsoniter.RawMessage `json:"payload"` // 未解码的报文内容
}

// 物模型报文定义
type Message struct {
	Type    string      `json:"type"`    // 报文类型
	Payload interface{} `json:"payload"` // 报文内容
}

// 事件或者方法的参数
type Args map[string]interface{}

// 调用结果参数
type Resp map[string]interface{}

// 未解析的事件或者方法参数
type RawArgs map[string]jsoniter.RawMessage

// 未解析的调用返回值
type RawResp map[string]jsoniter.RawMessage

// 状态
type State struct {
	Name string      `json:"name"` // 状态全名: 模型名/状态名
	Data interface{} `json:"data"` // 状态数据
}

// 事件
type Event struct {
	Name string `json:"name"` // 事件全名: 模型名/事件名
	Args Args   `json:"args"` // 事件参数
}

// 调用请求
type Call struct {
	Name string `json:"name"` // 方法全名: 模型名/方法名
	UUID string `json:"uuid"` // 调用请求的UUID
	Args Args   `json:"args"` // 调用请求的参数
}

// 调用结果
type Response struct {
	UUID     string `json:"uuid"`     // 调用的UUID
	Error    string `json:"error"`    // 错误提示信息
	Response Resp   `json:"response"` // 调用的结果
}

// 状态报文 报文内容定义
type StatePayload struct {
	Name string              `json:"name"` // 状态全名: 模型名/状态名
	Data jsoniter.RawMessage `json:"data"` // 状态原始数据
}

// 事件报文 报文内容定义
type EventPayload struct {
	Name string  `json:"name"` // 事件全名: 模型名/事件名
	Args RawArgs `json:"args"` // 事件参数
}

// 调用请求报文 报文内容定义
type CallPayload struct {
	Name string  `json:"name"` // 调用的全方法名: 模型名/方法名
	UUID string  `json:"uuid"` // 调用的UUID
	Args RawArgs `json:"args"` // 未解析的调用的参数
}

// 调用响应报文 报文内容定义
type ResponsePayload struct {
	UUID     string  `json:"uuid"`     // 响应的UUID
	Error    string  `json:"error"`    // 错误字符串
	Response RawResp `json:"response"` // 未解析的响应结果
}

// Must 保证编码必须无错误返回，否则会panic
func Must(msg []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return msg
}

// EncodeSubStateMsg 编码一个订阅类型为Type,订阅列表为items的状态订阅报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeSubStateMsg(Type int, items []string) ([]byte, error) {
	if items == nil {
		items = make([]string, 0)
	}
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

	msg := Message{
		Type:    typeStr,
		Payload: items,
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, _ := json.Marshal(msg)

	return ans, nil
}

// EncodeSubStateMsg 编码一个订阅类型为Type,订阅列表为items的事件订阅报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeSubEventMsg(Type int, items []string) ([]byte, error) {
	if items == nil {
		items = make([]string, 0)
	}
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

	msg := Message{
		Type:    typeStr,
		Payload: items,
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, _ := json.Marshal(msg)

	return ans, nil
}

// EncodeStateMsg 编码一个状态全名为stateName数据为data的状态报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeStateMsg(stateName string, data interface{}) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("nil data")
	}

	msg := Message{
		Type: "state",
		Payload: State{
			Name: stateName,
			Data: data,
		},
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode data failed")
	}

	return ans, nil
}

// EncodeEventMsg 编码一个事件全名为eventName参数为args的事件报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeEventMsg(eventName string, args Args) ([]byte, error) {
	if args == nil {
		args = Args{}
	}

	msg := Message{
		Type: "event",
		Payload: Event{
			Name: eventName,
			Args: args,
		},
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode event args failed")
	}

	return ans, nil
}

// EncodeCallMsg 编码一个方法全名为methodName,调用唯一标识为uuid,调用参数为args的调用请求报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeCallMsg(methodName string, uuid string, args Args) ([]byte, error) {
	if args == nil {
		args = Args{}
	}

	msg := Message{
		Type: "call",
		Payload: Call{
			Name: methodName,
			UUID: uuid,
			Args: args,
		},
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode call args failed")
	}

	return ans, nil
}

// EncodeRespMsg 编码一个调用标识为uuid,错误提示信息为errStr,响应结果为resp的调用结果报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeRespMsg(uuid string, errStr string, resp Resp) ([]byte, error) {
	if resp == nil {
		resp = Resp{}
	}

	msg := Message{
		Type: "response",
		Payload: Response{
			UUID:     uuid,
			Error:    errStr,
			Response: resp,
		},
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	ans, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode call response failed")
	}

	return ans, nil
}

// EncodeQueryMetaMsg 编码一个查询物模型元信息JSON报文, 返回JSON编码后的全报文数据
func EncodeQueryMetaMsg() []byte {
	return []byte(`{"type":"query-meta","payload":null}`)
}

// EncodeRawMsg 编码一个报文类型为Type,报文数据域为payload的JSON报文,
// 返回JSON编码后的全报文数据和错误信息
func EncodeRawMsg(Type string, payload jsoniter.RawMessage) ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	var value interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, fmt.Errorf("invalid payload")
	}

	msg := RawMessage{
		Type:    Type,
		Payload: payload,
	}

	ans, _ := json.Marshal(msg)
	return ans, nil
}
