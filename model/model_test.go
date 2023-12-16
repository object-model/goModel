package model

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"goModel/message"
	"goModel/meta"
	"net"
	"reflect"
	"testing"
)

// 模拟连接
type MockConn struct {
	mock.Mock
}

func (m *MockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) ReadMsg() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockConn) WriteMsg(msg []byte) error {
	args := m.Called(msg)
	return args.Error(0)
}

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

type motor struct {
	Rov  int `json:"rov"`
	Cur  int `json:"cur"`
	Temp int `json:"temp"`
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

type ModelTestSuite struct {
	suite.Suite
	server    *Model
	mockConn1 *MockConn
	mockConn2 *MockConn
}

func (s *ModelTestSuite) SetupSuite() {
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})

	require.Nil(s.T(), err)
	s.server = server
}

func (s *ModelTestSuite) SetupTest() {
	s.mockConn1 = new(MockConn)
	s.mockConn2 = new(MockConn)
}

func (s *ModelTestSuite) TestPushState() {
	state1 := tpqsInfo{
		QsState:  "erecting",
		HpSwitch: false,
		QsAngle:  90,
		Errors:   []errorInfo{},
	}

	msg1 := message.Must(message.EncodeStateMsg("A/car/#1/tpqs/tpqsInfo", state1))
	msg2 := message.Must(message.EncodeStateMsg("A/car/#1/tpqs/gear", 1))

	s.mockConn1.On("WriteMsg", msg1).Return(nil)
	s.mockConn1.On("WriteMsg", msg2).Return(nil)
	s.mockConn2.On("WriteMsg", msg2).Return(nil)

	conn1 := newConn(s.server, s.mockConn1)
	conn2 := newConn(s.server, s.mockConn2)

	conn1.pubStates["A/car/#1/tpqs/tpqsInfo"] = struct{}{}
	conn1.pubStates["A/car/#1/tpqs/gear"] = struct{}{}
	conn2.pubStates["A/car/#1/tpqs/gear"] = struct{}{}

	s.server.allConn[conn1] = struct{}{}
	s.server.allConn[conn2] = struct{}{}

	err := s.server.PushState("tpqsInfo", state1, false)
	require.Nil(s.T(), err)

	err = s.server.PushState("gear", uint(1), false)
	require.Nil(s.T(), err)

	s.mockConn1.AssertExpectations(s.T())
	s.mockConn2.AssertExpectations(s.T())
}

func (s *ModelTestSuite) TestPushState_Error() {
	err := s.server.PushState("unknown", 123, true)
	assert.EqualValues(s.T(), errors.New("NO state \"unknown\""), err, "不存在的状态")

	err = s.server.PushState("gear", "123", true)
	assert.EqualValues(s.T(), errors.New("type unmatched"), err, "不符合元信息的状态")
}

func (s *ModelTestSuite) TestPushEvent() {
	action := message.Args{
		"motors": [4]motor{
			{
				Rov:  1200,
				Cur:  28888,
				Temp: 41,
			},

			{
				Rov:  1300,
				Cur:  29888,
				Temp: 42,
			},

			{
				Rov:  1400,
				Cur:  29988,
				Temp: 43,
			},

			{
				Rov:  1100,
				Cur:  27888,
				Temp: 40,
			},
		},

		"qsAngle": 45,
	}

	msg1 := message.Must(message.EncodeEventMsg("A/car/#1/tpqs/qsMotorOverCur", message.Args{}))
	msg2 := message.Must(message.EncodeEventMsg("A/car/#1/tpqs/qsAction", action))

	s.mockConn1.On("WriteMsg", msg1).Return(nil)
	s.mockConn1.On("WriteMsg", msg2).Return(nil)
	s.mockConn2.On("WriteMsg", msg2).Return(nil)

	conn1 := newConn(s.server, s.mockConn1)
	conn2 := newConn(s.server, s.mockConn2)

	conn1.pubEvents["A/car/#1/tpqs/qsMotorOverCur"] = struct{}{}
	conn1.pubEvents["A/car/#1/tpqs/qsAction"] = struct{}{}
	conn2.pubEvents["A/car/#1/tpqs/qsAction"] = struct{}{}

	s.server.allConn[conn1] = struct{}{}
	s.server.allConn[conn2] = struct{}{}

	err := s.server.PushEvent("qsMotorOverCur", message.Args{}, true)
	require.Nil(s.T(), err)

	err = s.server.PushEvent("qsAction", action, true)
	require.Nil(s.T(), err)

	s.mockConn1.AssertExpectations(s.T())
	s.mockConn2.AssertExpectations(s.T())
}

func (s *ModelTestSuite) TestPushEvent_Error() {
	err := s.server.PushEvent("unknown", message.Args{}, true)
	assert.EqualValues(s.T(), errors.New("NO event \"unknown\""), err, "不存在的事件")

	err = s.server.PushEvent("qsAction", message.Args{"qsAngle": 90}, true)
	assert.EqualValues(s.T(), errors.New("arg \"motors\": missing"), err, "事件缺失参数")
}

func TestModel(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}
