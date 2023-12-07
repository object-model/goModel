package message

import (
	"errors"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMust(t *testing.T) {
	require.Panics(t, func() {
		Must(nil, errors.New("encode data failed"))
	})
	require.NotPanics(t, func() {
		msg := []byte(`{"type":"state","payload":100}`)
		ans := Must(msg, nil)
		assert.Equal(t, msg, ans)
	})
}

func TestEncodeSubStateMsg(t *testing.T) {
	type TestCase struct {
		subType  int
		items    []string
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			subType:  5,
			items:    []string{},
			wantData: nil,
			wantErr:  errors.New("invalid Type"),
			desc:     "无效的订阅类型",
		},

		{
			subType:  SetSub,
			items:    nil,
			wantData: []byte(`{"type":"set-subscribe-state","payload":[]}`),
			wantErr:  nil,
			desc:     "序列化成功--列表为nil",
		},

		{
			subType:  AddSub,
			items:    []string{},
			wantData: []byte(`{"type":"add-subscribe-state","payload":[]}`),
			wantErr:  nil,
			desc:     "序列化成功--列表为空",
		},

		{
			subType:  RemoveSub,
			items:    []string{"A/state1", "A/state2"},
			wantData: []byte(`{"type":"remove-subscribe-state","payload":["A/state1","A/state2"]}`),
			wantErr:  nil,
			desc:     "序列化成功--删除订阅",
		},

		{
			subType:  ClearSub,
			items:    []string{"A/state1", "A/state2"},
			wantData: []byte(`{"type":"clear-subscribe-state","payload":["A/state1","A/state2"]}`),
			wantErr:  nil,
			desc:     "序列化成功--清空订阅",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeSubStateMsg(test.subType, test.items)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeSubEventMsg(t *testing.T) {
	type TestCase struct {
		subType  int
		items    []string
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			subType:  5,
			items:    []string{},
			wantData: nil,
			wantErr:  errors.New("invalid Type"),
			desc:     "无效的订阅类型",
		},

		{
			subType:  SetSub,
			items:    nil,
			wantData: []byte(`{"type":"set-subscribe-event","payload":[]}`),
			wantErr:  nil,
			desc:     "序列化成功--列表为nil",
		},

		{
			subType:  AddSub,
			items:    []string{},
			wantData: []byte(`{"type":"add-subscribe-event","payload":[]}`),
			wantErr:  nil,
			desc:     "序列化成功--列表为空",
		},

		{
			subType:  RemoveSub,
			items:    []string{"A/state1", "A/state2"},
			wantData: []byte(`{"type":"remove-subscribe-event","payload":["A/state1","A/state2"]}`),
			wantErr:  nil,
			desc:     "序列化成功--删除订阅",
		},

		{
			subType:  ClearSub,
			items:    []string{"A/state1", "A/state2"},
			wantData: []byte(`{"type":"clear-subscribe-event","payload":["A/state1","A/state2"]}`),
			wantErr:  nil,
			desc:     "序列化成功--清空订阅",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeSubEventMsg(test.subType, test.items)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeStateMsg(t *testing.T) {
	type TestCase struct {
		name     string
		data     interface{}
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			name:     "model/state",
			data:     nil,
			wantData: nil,
			wantErr:  errors.New("nil data"),
			desc:     "空数据",
		},

		{
			name:     "model/state",
			data:     make(chan int),
			wantData: nil,
			wantErr:  errors.New("encode data failed"),
			desc:     "不支持序列化的数据--管道",
		},

		{
			name:     "model/state",
			data:     make([]chan int, 10),
			wantData: nil,
			wantErr:  errors.New("encode data failed"),
			desc:     "不支持序列化的数据--管道切片",
		},

		{
			name:     "model/state",
			data:     func() {},
			wantData: nil,
			wantErr:  errors.New("encode data failed"),
			desc:     "不支持序列化的数据--函数类型",
		},

		{
			name:     "model/state",
			data:     123,
			wantData: []byte(`{"type":"state","payload":{"name":"model/state","data":123}}`),
			wantErr:  nil,
			desc:     "序列化成功--简单数据类型",
		},

		{
			name: "model/state",
			data: []interface{}{
				123, "abc", true,
			},
			wantData: []byte(`{"type":"state","payload":{"name":"model/state","data":[123,"abc",true]}}`),
			wantErr:  nil,
			desc:     "序列化成功--数组类型",
		},

		{
			name: "model/state",
			data: map[string]interface{}{
				"b": []interface{}{
					false, "hello", 3.14,
				},
				"a": 123,
				"A": "hello",
			},
			// 注意序列化后的map的key都排序了
			wantData: []byte(`{"type":"state","payload":{"name":"model/state","data":{"A":"hello","a":123,"b":[false,"hello",3.14]}}}`),
			wantErr:  nil,
			desc:     "序列化成功--map类型",
		},

		{
			name: "model/state",
			data: struct {
				B []interface{} `json:"B"`
				A int           `json:"a"`
				c string
			}{
				A: 123,
				B: []interface{}{
					false, "hello", 3.14,
				},
				c: "unexported",
			},
			wantData: []byte(`{"type":"state","payload":{"name":"model/state","data":{"B":[false,"hello",3.14],"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--struct类型",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeStateMsg(test.name, test.data)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeEventMsg(t *testing.T) {
	type TestCase struct {
		name     string
		args     Args
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			name:     "model/event",
			args:     nil,
			wantData: []byte(`{"type":"event","payload":{"name":"model/event","args":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为nil",
		},

		{
			name:     "model/event",
			args:     Args{},
			wantData: []byte(`{"type":"event","payload":{"name":"model/event","args":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为空",
		},

		{
			name: "model/event",
			args: Args{
				"a": func() {},
			},
			wantData: nil,
			wantErr:  errors.New("encode event args failed"),
			desc:     "不支持序列化的数据--函数类型",
		},

		{
			name: "model/event",
			args: Args{
				"a": make(chan int),
			},
			wantData: nil,
			wantErr:  errors.New("encode event args failed"),
			desc:     "不支持序列化的数据--管道类型",
		},

		{
			name: "model/event",
			args: Args{
				"a": 123,
				"A": 34.56,
			},
			wantData: []byte(`{"type":"event","payload":{"name":"model/event","args":{"A":34.56,"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--简单类型",
		},

		{
			name: "model/event",
			args: Args{
				"a": 123,
				"B": []interface{}{
					"hello",
					map[string]interface{}{
						"go":  "so easy",
						"c++": "hard to learn",
					},
				},
			},
			wantData: []byte(`{"type":"event","payload":{"name":"model/event","args":{"B":["hello",{"c++":"hard to learn","go":"so easy"}],"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--复杂类型",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeEventMsg(test.name, test.args)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeCallMsg(t *testing.T) {
	type TestCase struct {
		name     string
		uuid     string
		args     Args
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			name:     "model/QS",
			args:     nil,
			uuid:     "1",
			wantData: []byte(`{"type":"call","payload":{"name":"model/QS","uuid":"1","args":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为nil",
		},

		{
			name:     "model/QS",
			args:     Args{},
			uuid:     "2",
			wantData: []byte(`{"type":"call","payload":{"name":"model/QS","uuid":"2","args":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为空",
		},

		{
			name:     "model/QS",
			args:     Args{},
			uuid:     "2",
			wantData: []byte(`{"type":"call","payload":{"name":"model/QS","uuid":"2","args":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为空",
		},

		{
			name: "model/QS",
			args: Args{
				"a": make(chan int),
			},
			wantData: nil,
			wantErr:  errors.New("encode call args failed"),
			desc:     "不支持序列化的数据--管道类型",
		},

		{
			name: "model/QS",
			args: Args{
				"a": 123,
				"A": 34.56,
			},
			uuid:     "abc",
			wantData: []byte(`{"type":"call","payload":{"name":"model/QS","uuid":"abc","args":{"A":34.56,"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--简单类型",
		},

		{
			name: "model/QS",
			args: Args{
				"a": 123,
				"B": []interface{}{
					"hello",
					map[string]interface{}{
						"go":  "so easy",
						"c++": "hard to learn",
					},
				},
			},
			uuid:     "fff-e0",
			wantData: []byte(`{"type":"call","payload":{"name":"model/QS","uuid":"fff-e0","args":{"B":["hello",{"c++":"hard to learn","go":"so easy"}],"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--复杂类型",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeCallMsg(test.name, test.uuid, test.args)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeRespMsg(t *testing.T) {
	type TestCase struct {
		uuid     string
		errStr   string
		resp     Resp
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{
		{
			uuid:     "1",
			errStr:   "方法不存在",
			resp:     nil,
			wantData: []byte(`{"type":"response","payload":{"uuid":"1","error":"方法不存在","response":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为nil",
		},

		{
			uuid:     "2",
			errStr:   "方法不存在",
			resp:     Resp{},
			wantData: []byte(`{"type":"response","payload":{"uuid":"2","error":"方法不存在","response":{}}}`),
			wantErr:  nil,
			desc:     "序列化成功--参数为空",
		},

		{
			uuid:   "abc",
			errStr: "成功",
			resp: Resp{
				"a": func() {},
			},
			wantData: nil,
			wantErr:  errors.New("encode call response failed"),
			desc:     "不支持序列化的数据--函数类型",
		},

		{
			resp: Resp{
				"a": 123,
				"B": []interface{}{
					"hello",
					map[string]interface{}{
						"go":  "so easy",
						"c++": "hard to learn",
					},
				},
			},
			errStr:   "成功",
			uuid:     "fff-e0",
			wantData: []byte(`{"type":"response","payload":{"uuid":"fff-e0","error":"成功","response":{"B":["hello",{"c++":"hard to learn","go":"so easy"}],"a":123}}}`),
			wantErr:  nil,
			desc:     "序列化成功--复杂类型",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeRespMsg(test.uuid, test.errStr, test.resp)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}

func TestEncodeQueryMetaMsg(t *testing.T) {
	require.EqualValues(t, []byte(`{"type":"query-meta","payload":null}`), EncodeQueryMetaMsg())
}

func TestEncodeRawMsg(t *testing.T) {
	type TestCase struct {
		typeStr  string
		payload  jsoniter.RawMessage
		wantData []byte
		wantErr  error
		desc     string
	}

	testCases := []TestCase{

		{
			typeStr:  "query-meta",
			payload:  []byte(`{]`),
			wantData: nil,
			wantErr:  errors.New("invalid payload"),
			desc:     "编码失败--payload为无效JSON情况1",
		},

		{
			typeStr:  "query-meta",
			payload:  []byte(`{}[]`),
			wantData: nil,
			wantErr:  errors.New("invalid payload"),
			desc:     "编码失败--payload为无效JSON情况2",
		},

		{
			typeStr:  "query-meta",
			payload:  []byte(`123true"abc"`),
			wantData: nil,
			wantErr:  errors.New("invalid payload"),
			desc:     "编码失败--payload为无效JSON情况3",
		},

		{
			typeStr:  "state",
			payload:  []byte(`123e-1.2`),
			wantData: nil,
			wantErr:  errors.New("invalid payload"),
			desc:     "编码失败--payload为无效JSON情况4--无效的科学计数法",
		},

		{
			typeStr:  "query-meta",
			payload:  nil,
			wantData: nil,
			wantErr:  errors.New("invalid payload"),
			desc:     "编码失败--payload为nil",
		},

		{
			typeStr:  "query-meta",
			payload:  []byte(`null`),
			wantData: []byte(`{"type":"query-meta","payload":null}`),
			wantErr:  nil,
			desc:     "编码成功--null数据域",
		},

		{
			typeStr:  "state",
			payload:  []byte(`123e-1`),
			wantData: []byte(`{"type":"state","payload":123e-1}`),
			wantErr:  nil,
			desc:     "编码成功--科学计数法",
		},
	}

	for _, test := range testCases {
		gotData, gotErr := EncodeRawMsg(test.typeStr, test.payload)
		require.EqualValues(t, test.wantData, gotData, test.desc)
		require.EqualValues(t, test.wantErr, gotErr, test.desc)
	}
}
