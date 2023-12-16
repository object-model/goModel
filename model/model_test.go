package model

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"goModel/message"
	"goModel/meta"
	"io"
	"net"
	"testing"
)

// 模拟连接
type mockConn struct {
	mock.Mock
}

func (m *mockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *mockConn) ReadMsg() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockConn) WriteMsg(msg []byte) error {
	args := m.Called(msg)
	return args.Error(0)
}

type mockStateHandler struct {
	mock.Mock
}

func (m *mockStateHandler) OnState(modelName string, stateName string, data []byte) {
	m.Called(modelName, stateName, data)
}

type mockEventHandler struct {
	mock.Mock
}

func (m *mockEventHandler) OnEvent(modelName string, eventName string, args message.RawArgs) {
	m.Called(modelName, eventName, args)
}

type mockCloseHandler struct {
	mock.Mock
}

func (m *mockCloseHandler) OnClosed(reason string) {
	m.Called(reason)
}

type mockCallReqHandler struct {
	mock.Mock
}

func (m *mockCallReqHandler) OnCallReq(name string, args message.RawArgs) message.Resp {
	retAgs := m.Called(name, args)
	return retAgs.Get(0).(message.Resp)
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

func TestLoadFromFileFailed(t *testing.T) {
	_, err := LoadFromFile("unknown.json", meta.TemplateParam{
		"group": " A ",
		"id":    "#1",
	})

	assert.NotNil(t, err, "加载不存在的文件")
}

// ModelTestSuite
type ModelTestSuite struct {
	suite.Suite
	server *Model // 服务端物模型
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
	s.server.allConn = make(map[*Connection]struct{})
}

// TestPushState 测试推送状态报文成功的情况
func (s *ModelTestSuite) TestPushState() {

	mockConn1 := new(mockConn)
	mockConn2 := new(mockConn)

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

	conn1 := newConn(s.server, mockConn1)
	conn2 := newConn(s.server, mockConn2)

	conn1.pubStates["A/car/#1/tpqs/tpqsInfo"] = struct{}{}
	conn1.pubStates["A/car/#1/tpqs/gear"] = struct{}{}
	conn2.pubStates["A/car/#1/tpqs/gear"] = struct{}{}

	s.server.allConn[conn1] = struct{}{}
	s.server.allConn[conn2] = struct{}{}

	err := s.server.PushState("tpqsInfo", state1, false)
	require.Nil(s.T(), err)

	err = s.server.PushState("gear", uint(1), false)
	require.Nil(s.T(), err)

	mockConn1.AssertExpectations(s.T())
	mockConn2.AssertExpectations(s.T())
}

// TestPushState_Error 测试推送状态报文出错的情况
func (s *ModelTestSuite) TestPushState_Error() {
	err := s.server.PushState("unknown", 123, true)
	assert.EqualValues(s.T(), errors.New("NO state \"unknown\""), err, "不存在的状态")

	err = s.server.PushState("gear", "123", true)
	assert.EqualValues(s.T(), errors.New("type unmatched"), err, "不符合元信息的状态")
}

// TestPushEvent 测试推送事件报文成功的情况
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

	mockConn1 := new(mockConn)
	mockConn2 := new(mockConn)

	msg1 := message.Must(message.EncodeEventMsg("A/car/#1/tpqs/qsMotorOverCur", message.Args{}))
	msg2 := message.Must(message.EncodeEventMsg("A/car/#1/tpqs/qsAction", action))

	mockConn1.On("WriteMsg", msg1).Return(nil)
	mockConn1.On("WriteMsg", msg2).Return(nil)
	mockConn2.On("WriteMsg", msg2).Return(nil)

	conn1 := newConn(s.server, mockConn1)
	conn2 := newConn(s.server, mockConn2)

	conn1.pubEvents["A/car/#1/tpqs/qsMotorOverCur"] = struct{}{}
	conn1.pubEvents["A/car/#1/tpqs/qsAction"] = struct{}{}
	conn2.pubEvents["A/car/#1/tpqs/qsAction"] = struct{}{}

	s.server.allConn[conn1] = struct{}{}
	s.server.allConn[conn2] = struct{}{}

	err := s.server.PushEvent("qsMotorOverCur", message.Args{}, true)
	require.Nil(s.T(), err)

	err = s.server.PushEvent("qsAction", action, true)
	require.Nil(s.T(), err)

	mockConn1.AssertExpectations(s.T())
	mockConn2.AssertExpectations(s.T())
}

func (s *ModelTestSuite) TestPushEvent_Error() {
	err := s.server.PushEvent("unknown", message.Args{}, true)
	assert.EqualValues(s.T(), errors.New("NO event \"unknown\""), err, "不存在的事件")

	err = s.server.PushEvent("qsAction", message.Args{"qsAngle": 90}, true)
	assert.EqualValues(s.T(), errors.New("arg \"motors\": missing"), err, "事件缺失参数")
}

// TestDealInvalidJsonMsg 测试收到无效的JSON数据的情况
func (s *ModelTestSuite) TestDealInvalidJsonMsg() {
	mockedConn := new(mockConn)

	conn1 := newConn(s.server, mockedConn, WithClosedFunc(func(reason string) {
		fmt.Println("onClosed:", reason)
		assert.Contains(s.T(), reason, "decode json:", "解码JSON错误")
	}))

	mockedConn.On("ReadMsg").Return([]byte(`{{123]`), nil).Once()
	mockedConn.On("Close").Return(errors.New("already closed")).Once()

	s.server.dealConn(conn1)

	assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

	mockedConn.AssertExpectations(s.T())
}

// TestDealSubStateMsg 测试状态订阅报文的处理逻辑
func (s *ModelTestSuite) TestDealSubStateMsg() {

	type TestCase struct {
		initial map[string]struct{} // 初始的状态发布表
		kind    int                 // 更新类型
		states  []string            // 状态列表
		wanted  map[string]struct{} // 期望的状态发布表
		desc    string              // 用例描述
	}

	testCases := []TestCase{
		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			kind: message.SetSub,
			states: []string{
				"A/car/#1/tpqs/tpqsInfo",
				"A/car/#1/tpqs/gear",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo": {},
				"A/car/#1/tpqs/gear":     {},
			},
			desc: "设置状态订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			kind: message.AddSub,
			states: []string{
				"A/car/#1/tpqs/tpqsInfo",
				"A/car/#1/tpqs/gear",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
				"A/car/#1/tpqs/gear":      {},
			},
			desc: "添加状态订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			kind: message.RemoveSub,
			states: []string{
				"A/car/#1/tpqs/tpqsInfo",
				"A/car/#1/tpqs/gear",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/powerInfo": {},
			},
			desc: "取消状态订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			kind:   message.ClearSub,
			states: nil,
			wanted: map[string]struct{}{},
			desc:   "清空状态订阅报文",
		},
	}

	for _, test := range testCases {
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)
		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose))
		conn.pubStates = test.initial

		// 测试报文数据
		subStateMsg := message.Must(message.EncodeSubStateMsg(test.kind, test.states))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return(subStateMsg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.EqualValues(s.T(), test.wanted, conn.pubStates, test.desc)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}
}

// TestDealSubStateMsg_Error 测试解析状态订阅报文错误的情况
func (s *ModelTestSuite) TestDealSubStateMsg_Error() {

	type TestCase struct {
		initial map[string]struct{} // 初始的状态发布表
		msg     string              // 测试报文数据
		wanted  map[string]struct{} // 期望的状态发布表
		desc    string              // 用例描述
	}

	testCases := []TestCase{
		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			msg: `{"type":"set-subscribe-state","payload":["a", 123]}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			desc: "设置状态订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			msg: `{"type":"add-subscribe-state","payload":{}}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			desc: "添加状态订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			msg: `{"type":"remove-subscribe-state","payload":["A", "B", {}]}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/tpqsInfo":  {},
				"A/car/#1/tpqs/powerInfo": {},
			},
			desc: "删除状态订阅报文",
		},
	}

	for _, test := range testCases {
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)
		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose))
		conn.pubStates = test.initial

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return([]byte(test.msg), nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.EqualValues(s.T(), test.wanted, conn.pubStates, test.desc)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}
}

// TestDealSubEventMsg 测试事件订阅报文的处理逻辑
func (s *ModelTestSuite) TestDealSubEventMsg() {

	type TestCase struct {
		initial map[string]struct{} // 初始的事件发布表
		kind    int                 // 更新类型
		events  []string            // 事件列表
		wanted  map[string]struct{} // 期望的事件发布表
		desc    string              // 用例描述
	}

	testCases := []TestCase{
		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			kind: message.SetSub,
			events: []string{
				"A/car/#1/tpqs/qsAction",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsAction": {},
			},
			desc: "设置事件订阅报文",
		},

		{
			initial: map[string]struct{}{},
			kind:    message.AddSub,
			events: []string{
				"A/car/#1/tpqs/qsMotorOverCur",
				"A/car/#1/tpqs/qsAction",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			desc: "添加事件订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			kind: message.RemoveSub,
			events: []string{
				"A/car/#1/tpqs/qsMotorOverCur",
			},
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsAction": {},
			},
			desc: "取消事件订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			kind:   message.ClearSub,
			events: nil,
			wanted: map[string]struct{}{},
			desc:   "清空事件订阅报文",
		},
	}

	for _, test := range testCases {
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)
		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose))
		conn.pubEvents = test.initial

		// 测试报文数据
		subEventMsg := message.Must(message.EncodeSubEventMsg(test.kind, test.events))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return(subEventMsg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.EqualValues(s.T(), test.wanted, conn.pubEvents, test.desc)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}
}

// TestDealSubEventMsg_Error 测试解析事件订阅报文错误的情况
func (s *ModelTestSuite) TestDealSubEventMsg_Error() {

	type TestCase struct {
		initial map[string]struct{} // 初始的事件发布表
		msg     string              // 测试报文数据
		wanted  map[string]struct{} // 期望的状态发布表
		desc    string              // 用例描述
	}

	testCases := []TestCase{
		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
			},
			msg: `{"type":"set-subscribe-event","payload":["a", 123]}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
			},
			desc: "设置事件订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			msg: `{"type":"add-subscribe-event","payload":{}}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			desc: "添加事件订阅报文",
		},

		{
			initial: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			msg: `{"type":"remove-subscribe-event","payload":["A", "B", {}]}`,
			wanted: map[string]struct{}{
				"A/car/#1/tpqs/qsMotorOverCur": {},
				"A/car/#1/tpqs/qsAction":       {},
			},
			desc: "删除状态订阅报文",
		},
	}

	for _, test := range testCases {
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)
		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose))
		conn.pubStates = test.initial

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return([]byte(test.msg), nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.EqualValues(s.T(), test.wanted, conn.pubStates, test.desc)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}
}

// TestDealQueryMetaMsg 测试元信息查询报文处理逻辑
func (s *ModelTestSuite) TestDealQueryMetaMsg() {
	mockOnClose := new(mockCloseHandler)
	mockedConn := new(mockConn)

	conn1 := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose))

	metaMsg := message.Must(message.EncodeRawMsg("meta-info", s.server.Meta().ToJSON()))

	mockOnClose.On("OnClosed", io.EOF.Error()).Once()
	mockedConn.On("ReadMsg").Return(message.EncodeQueryMetaMsg(), nil).Once()
	mockedConn.On("WriteMsg", metaMsg).Return(nil).Once()
	mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
	mockedConn.On("Close").Return(errors.New("already closed")).Once()

	s.server.dealConn(conn1)

	assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

	mockedConn.AssertExpectations(s.T())
	mockOnClose.AssertExpectations(s.T())
}

func (s *ModelTestSuite) TestDialTcp() {
	go func() {
		_ = s.server.ListenServeTCP(":61234")
	}()

	client := NewEmptyModel()
	conn, err := client.DialTcp("localhost:61234",
		WithStateFunc(func(modelName string, stateName string, data []byte) {

		}),
		WithEventFunc(func(modelName string, eventName string, args message.RawArgs) {

		}),
		WithClosedFunc(func(reason string) {

		}),
		WithEventBuffSize(512),
		WithStateBuffSize(1024),
	)
	require.Nil(s.T(), err, "连接未成功")

	assert.Equal(s.T(), 1024, cap(conn.statesChan))
	assert.Equal(s.T(), 512, cap(conn.eventsChan))

}

func TestModel(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}
