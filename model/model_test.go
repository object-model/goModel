package model

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"goModel/message"
	"goModel/meta"
	"goModel/rawConn/mockConn"
	"reflect"
	"testing"
)

type errorInfo struct {
	Code uint   `json:"code"`
	Msg  string `json:"msg"`
}
type tpqsInfo struct {
	QsState  string      `json:"qsState"`
	HpSwitch bool        `json:"hpSwitch"`
	QsAngle  float64     `json:"qsAngle"`
	Errors   []errorInfo `json:"errors"`
}

func TestNew(t *testing.T) {
	onCall := func(name string, args message.RawArgs) message.Resp {
		return message.Resp{}
	}
	metaInfo := meta.NewEmptyMeta()
	m := New(metaInfo, WithVerifyResp(), WithCallReq(onCall))

	assert.EqualValues(t, metaInfo, m.Meta())
	assert.True(t, m.verifyResp, "开启响应校验")
	assert.Equal(t, reflect.ValueOf(onCall).Pointer(), reflect.ValueOf(m.callReqHandler).Pointer(),
		"注册的调用请求回调函数")
	assert.EqualValues(t, make(map[*Connection]struct{}), m.allConn, "连接初始为空")
}

func TestLoadFromFileFailed(t *testing.T) {
	_, err := LoadFromFile("unknown.json", meta.TemplateParam{
		"group": " A ",
		"id":    "#1",
	})

	assert.NotNil(t, err, "加载不存在的文件")
}

func TestLoadFromFile(t *testing.T) {
	onCall := func(name string, args message.RawArgs) message.Resp {
		return message.Resp{}
	}
	m, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": " A ",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReq(onCall))

	require.Nil(t, err)

	assert.Equal(t, true, m.verifyResp, "开启响应校验")
	assert.Equal(t, reflect.ValueOf(onCall).Pointer(), reflect.ValueOf(m.callReqHandler).Pointer(),
		"注册的调用请求回调函数")
}

func TestModel_PushState(t *testing.T) {
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})

	require.Nil(t, err)

	mockConn1 := new(mockConn.MockConn)
	mockConn2 := new(mockConn.MockConn)

	state1 := tpqsInfo{
		QsState:  "erecting",
		HpSwitch: false,
		QsAngle:  90,
		Errors:   []errorInfo{},
	}

	msg1 := message.Must(message.EncodeStateMsg("A/car/#1/tpqs/tpqsInfo", state1))
	msg2 := message.Must(message.EncodeStateMsg("A/car/#1/tpqs/gear", 1))

	mockConn1.On("WriteMsg", msg1).Return(nil)
	mockConn1.On("WriteMsg", msg2).Return(nil)
	mockConn2.On("WriteMsg", msg2).Return(nil)

	conn1 := newConn(server, mockConn1)
	conn2 := newConn(server, mockConn2)

	conn1.pubStates["A/car/#1/tpqs/tpqsInfo"] = struct{}{}
	conn1.pubStates["A/car/#1/tpqs/gear"] = struct{}{}
	conn2.pubStates["A/car/#1/tpqs/gear"] = struct{}{}

	server.allConn[conn1] = struct{}{}
	server.allConn[conn2] = struct{}{}

	err = server.PushState("tpqsInfo", state1, false)
	require.Nil(t, err)

	err = server.PushState("gear", uint(1), false)
	require.Nil(t, err)

	mockConn1.AssertExpectations(t)
	mockConn2.AssertExpectations(t)
}

func TestModel_PushState_Error(t *testing.T) {
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})

	require.Nil(t, err)

	err = server.PushState("unknown", 123, true)
	assert.EqualValues(t, errors.New("NO state \"unknown\""), err, "不存在的状态")

	err = server.PushState("gear", "123", true)
	assert.EqualValues(t, errors.New("type unmatched"), err, "不符合元信息的状态")
}
