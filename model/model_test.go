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
	"reflect"
	"testing"
	"time"
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

// TestWithVerifyResp 测试开启物模型的响应校验选项
func TestWithVerifyResp(t *testing.T) {
	m := &Model{}
	WithVerifyResp()(m)
	assert.True(t, m.verifyResp, "开启校验返回值")
}

// TestWithCallReqHandler 测试配置物模型的调用请求回调处理对象
func TestWithCallReqHandler(t *testing.T) {
	m := &Model{}
	mockCallReq := new(mockCallReqHandler)
	WithCallReqHandler(mockCallReq)(m)
	assert.Equal(t, mockCallReq, m.callReqHandler, "配置调用请求回调处理对象")
}

func TestWithCallReqFunc(t *testing.T) {
	m := &Model{}
	onCall := func(name string, args message.RawArgs) message.Resp {
		return message.Resp{}
	}
	WithCallReqFunc(onCall)(m)
	assert.Equal(t, reflect.ValueOf(onCall).Pointer(),
		reflect.ValueOf(m.callReqHandler).Pointer(),
		"配置调用请求回调处理函数")
}

// TestLoadFromFileFailed 测试从文件加载模型失败情况
func TestLoadFromFileFailed(t *testing.T) {
	_, err := LoadFromFile("unknown.json", meta.TemplateParam{
		"group": " A ",
		"id":    "#1",
	})

	assert.NotNil(t, err, "加载不存在的文件")
}

// ModelTestSuite
type StateEventSuite struct {
	suite.Suite
	server *Model // 服务端物模型
}

func (s *StateEventSuite) SetupSuite() {
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})

	require.Nil(s.T(), err)
	s.server = server
}

func (s *StateEventSuite) SetupTest() {
	s.server.allConn = make(map[*Connection]struct{})
}

// TestPushState 测试推送状态报文成功的情况
func (s *StateEventSuite) TestPushState() {

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
func (s *StateEventSuite) TestPushState_Error() {
	err := s.server.PushState("unknown", 123, true)
	assert.EqualValues(s.T(), errors.New("NO state \"unknown\""), err, "不存在的状态")

	err = s.server.PushState("gear", "123", true)
	assert.EqualValues(s.T(), errors.New("type unmatched"), err, "不符合元信息的状态")
}

// TestPushEvent 测试推送事件报文成功的情况
func (s *StateEventSuite) TestPushEvent() {
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

func (s *StateEventSuite) TestPushEvent_Error() {
	err := s.server.PushEvent("unknown", message.Args{}, true)
	assert.EqualValues(s.T(), errors.New("NO event \"unknown\""), err, "不存在的事件")

	err = s.server.PushEvent("qsAction", message.Args{"qsAngle": 90}, true)
	assert.EqualValues(s.T(), errors.New("arg \"motors\": missing"), err, "事件缺失参数")
}

// TestDealInvalidJsonMsg 测试收到无效的JSON数据的情况
func (s *StateEventSuite) TestDealInvalidJsonMsg() {
	wantErrStr := "connection closed for: decode json:"
	mockedConn := new(mockConn)
	waiter := &RespWaiter{
		got: make(chan struct{}),
	}
	conn1 := newConn(s.server, mockedConn, WithClosedFunc(func(reason string) {
		fmt.Println("onClosed:", reason)
		assert.Contains(s.T(), reason, "decode json: ", "解码JSON错误时关闭回调函数行为")
	}))
	conn1.respWaiters = map[string]*RespWaiter{
		"123": waiter,
	}

	mockedConn.On("ReadMsg").Return([]byte(`{{123]`), nil).Once()
	mockedConn.On("Close").Return(errors.New("already closed")).Once()

	s.server.dealConn(conn1)

	assert.Contains(s.T(), conn1.peerMetaErr.Error(), wantErrStr, "解码JSON错误时对端元信息错误信息")

	select {
	case <-waiter.got:
		assert.Contains(s.T(), waiter.err.Error(), wantErrStr)
		assert.EqualValues(s.T(), message.RawResp{}, waiter.resp)
	default:
		assert.Fail(s.T(), "等待器未唤醒")
	}

	assert.Len(s.T(), conn1.respWaiters, 0, "响应等待器已清空")
	assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

	mockedConn.AssertExpectations(s.T())
}

// TestDealSubStateMsg 测试状态订阅报文的处理逻辑
func (s *StateEventSuite) TestDealSubStateMsg() {

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
func (s *StateEventSuite) TestDealSubStateMsg_Error() {

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
func (s *StateEventSuite) TestDealSubEventMsg() {

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
func (s *StateEventSuite) TestDealSubEventMsg_Error() {

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

// TestDealStateMsg 测试收到正常的状态报文
func (s *StateEventSuite) TestDealStateMsg() {
	mockOnClose := new(mockCloseHandler)
	mockOnState := new(mockStateHandler)
	mockedConn := new(mockConn)

	// 测试报文数据
	msg := message.Must(message.EncodeStateMsg("model/state", map[string]interface{}{
		"a": 123,
		"b": []string{
			"Go", "C++", "Python",
		},
	}))

	// 状态回调期望的数据
	wanted := []byte(`{"a":123,"b":["Go","C++","Python"]}`)

	conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose),
		WithStateHandler(mockOnState))

	mockOnClose.On("OnClosed", io.EOF.Error()).Once()
	mockOnState.On("OnState", "model", "state", wanted).Once()

	mockedConn.On("ReadMsg").Return(msg, nil).Once()
	mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
	mockedConn.On("Close").Return(errors.New("already closed")).Once()

	s.server.dealConn(conn)

	assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

	mockedConn.AssertExpectations(s.T())
	mockOnState.AssertExpectations(s.T())
	mockOnClose.AssertExpectations(s.T())
}

// TestDealInvalidStateMsg 测试收到无效的状态报文
func (s *StateEventSuite) TestDealInvalidStateMsg() {
	type TestCase struct {
		msg  []byte // 测试报文数据
		desc string // 用例描述
	}

	testCases := []TestCase{
		{
			msg:  []byte(`{"type":"state","payload":[]}`),
			desc: "payload不是对象",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"name":123,"data":"hello world"}}`),
			desc: "name字段不是字符串",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"data":123}}`),
			desc: "payload缺失name字段",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"name":"    \f\t","data":123}}`),
			desc: "name字段去除空格后为空",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"name":"car/state"}}`),
			desc: "payload缺失data字段",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"name":"car/state","args":null}}`),
			desc: "args字段为null",
		},

		{
			msg:  []byte(`{"type":"state","payload":{"name":"invalid format","data":"hello world"}}`),
			desc: "状态名称name不包含/",
		},
	}

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnClose := new(mockCloseHandler)
		mockOnState := new(mockStateHandler)
		mockedConn := new(mockConn)

		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose),
			WithStateHandler(mockOnState))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnState.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}

}

// TestDealEventMsg 测试收到正常的事件报文
func (s *StateEventSuite) TestDealEventMsg() {
	mockOnClose := new(mockCloseHandler)
	mockOnEvent := new(mockEventHandler)
	mockedConn := new(mockConn)

	// 测试报文数据
	msg := message.Must(message.EncodeEventMsg("model/event", message.Args{
		"a": 123,
		"b": []string{
			"Go", "C++", "Python",
		},
	}))

	// 事件回调期望的参数
	wanted := message.RawArgs{
		"a": []byte(`123`),
		"b": []byte(`["Go","C++","Python"]`),
	}

	conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose),
		WithEventHandler(mockOnEvent))

	mockOnClose.On("OnClosed", io.EOF.Error()).Once()
	mockOnEvent.On("OnEvent", "model", "event", wanted).Once()

	mockedConn.On("ReadMsg").Return(msg, nil).Once()
	mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
	mockedConn.On("Close").Return(errors.New("already closed")).Once()

	s.server.dealConn(conn)

	assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

	mockedConn.AssertExpectations(s.T())
	mockOnEvent.AssertExpectations(s.T())
	mockOnClose.AssertExpectations(s.T())
}

// TestDealInvalidStateMsg 测试收到无效的状态报文
func (s *StateEventSuite) TestDealInvalidEventMsg() {
	type TestCase struct {
		msg  []byte // 测试报文数据
		desc string // 用例描述
	}

	testCases := []TestCase{
		{
			msg:  []byte(`{"type":"event","payload":[]}`),
			desc: "payload不是对象",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":123,"args":"hello world"}}`),
			desc: "name字段不是字符串",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":"invalid format","args":{"a":1, "b":true}}}`),
			desc: "状态名称name不包含/",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":"model/event"}}`),
			desc: "缺失args字段",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":"","args":{}}}`),
			desc: "name字段为空",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":"    ","args":{}}}`),
			desc: "name字段去除空格后为空",
		},

		{
			msg:  []byte(`{"type":"event","payload":{"name":"123","args":null}}`),
			desc: "args字段为null",
		},
	}

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnClose := new(mockCloseHandler)
		mockOnEvent := new(mockEventHandler)
		mockedConn := new(mockConn)

		conn := newConn(s.server, mockedConn, WithClosedHandler(mockOnClose),
			WithEventHandler(mockOnEvent))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		s.server.dealConn(conn)

		assert.Len(s.T(), s.server.allConn, 0, "管理的连接必须为空")

		mockedConn.AssertExpectations(s.T())
		mockOnEvent.AssertExpectations(s.T())
		mockOnClose.AssertExpectations(s.T())
	}

}

// TestDealQueryMetaMsg 测试元信息查询报文处理逻辑
func (s *StateEventSuite) TestDealQueryMetaMsg() {
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

// TestAboutStateEvent 测试推送状态、事件、状态和事件报文、状态和事件订阅报文的处理逻辑
func TestAboutStateEvent(t *testing.T) {
	suite.Run(t, new(StateEventSuite))
}

// TestDealCallMsg 测试有效调用请求报文
func TestDealCallMsg(t *testing.T) {

	type TestCase struct {
		msg        []byte          // 测试报文数据
		resp       message.Resp    // 调用请求返回值
		hasOnCall  bool            // 是否注册了调用请求回调
		verifyResp bool            // 是否对象响应校验
		wantMsg    []byte          // 期望的响应报文数据
		wantArgs   message.RawArgs // 调用请求回调期望的调用参数
		desc       string          // 用例描述
	}

	testCases := []TestCase{
		{
			msg:     []byte(`{"type":"call","payload":{"name":"invalid format","uuid":"123456","args":{}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"fullName is invalid format","response":{}}}`),
			desc:    "调用方法名无效",
		},

		{
			msg:     []byte(`{"type":"call","payload":{"name":"unknown/QS","uuid":"123456","args":{}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"modelName \"unknown\": unmatched","response":{}}}`),
			desc:    "调用的模型名称不匹配",
		},

		{
			msg:     []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/unknown","uuid":"123456","args":{}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"NO method \"unknown\"","response":{}}}`),
			desc:    "调用的方法不存在",
		},

		{
			msg:     []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"arg \"angle\": missing","response":{}}}`),
			desc:    "调用的参数不符合元信息---参数缺失",
		},

		{
			msg:     []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"arg \"angle\": missing","response":{}}}`),
			desc:    "调用的参数不符合元信息---参数缺失",
		},

		{
			msg:     []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"fast"}}}`),
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"NO callback","response":{}}}`),
			desc:    "没有注册调用请求回调",
		},

		{
			msg:       []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"fast"}}}`),
			hasOnCall: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			resp:    nil,
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"","response":{}}}`),
			desc:    "没有开启响应校验---回调返回的是nil",
		},

		{
			msg:       []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"fast"}}}`),
			hasOnCall: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			resp: message.Resp{
				"res": true,
				"msg": "执行成功",
			},
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"","response":{"msg":"执行成功","res":true}}}`),
			desc:    "没有开启响应校验直接采信回调输出",
		},

		{
			msg:        []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"middle"}}}`),
			hasOnCall:  true,
			verifyResp: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res": true,
				"msg": "执行成功",
			},
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"response \"time\": missing","response":{"msg":"执行成功","res":true}}}`),
			desc:    "开启响应校验---返回值不符合元信息---返回值缺失",
		},

		{
			msg:        []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"middle"}}}`),
			hasOnCall:  true,
			verifyResp: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": 100,
				"code": 0,
			},
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"response \"time\": type unmatched","response":{"code":0,"msg":"执行成功","res":true,"time":100}}}`),
			desc:    "开启响应校验---返回值不符合元信息---返回值类型不匹配",
		},

		{
			msg:        []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"middle"}}}`),
			hasOnCall:  true,
			verifyResp: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  false,
				"msg":  "未知执行结果",
				"time": uint(100),
				"code": 5,
			},
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"response \"code\": 5 NOT in option","response":{"code":5,"msg":"未知执行结果","res":false,"time":100}}}`),
			desc:    "开启响应校验---返回值不符合元信息---返回值不再可选项中",
		},

		{
			msg:        []byte(`{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123456","args":{"angle":90,"speed":"middle"}}}`),
			hasOnCall:  true,
			verifyResp: true,
			wantArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  false,
				"msg":  "驱动器未上电",
				"time": uint(100),
				"code": 2,
			},
			wantMsg: []byte(`{"type":"response","payload":{"uuid":"123456","error":"","response":{"code":2,"msg":"驱动器未上电","res":false,"time":100}}}`),
			desc:    "开启响应校验---无错误",
		},
	}

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnCall := new(mockCallReqHandler)
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)

		var opts []ModelOption
		if test.hasOnCall {
			opts = append(opts, WithCallReqHandler(mockOnCall))
		}
		if test.verifyResp {
			opts = append(opts, WithVerifyResp())
		}

		server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
			"group": "A",
			"id":    "#1",
		}, opts...)

		require.Nil(t, err)

		conn := newConn(server, mockedConn, WithClosedHandler(mockOnClose))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()

		if test.wantArgs != nil {
			mockOnCall.On("OnCallReq", "QS", test.wantArgs).Return(test.resp).Once()
		}

		mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		mockedConn.On("WriteMsg", test.wantMsg).Return(nil).Once()

		// NOTE: 目的是确保在需要调用回调是, 等回调返回再沿着
		// NOTE: 否则由于回调还没来的及触发, 而导致提前验证不通过
		mockedConn.On("ReadMsg").After(time.Second/10).Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		server.dealConn(conn)

		mockedConn.AssertExpectations(t)
		mockOnCall.AssertExpectations(t)
		mockOnClose.AssertExpectations(t)
	}
}

// TestDealInvalidCallMsg 测试无效调用请求报文
func TestDealInvalidCallMsg(t *testing.T) {
	type TestCase struct {
		msg  []byte // 测试报文数据
		desc string // 用例描述
	}

	testCases := []TestCase{
		{
			msg:  []byte(`{"type":"call","payload":[]}`),
			desc: "payload不是对象",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":123}}`),
			desc: "调用请求的name字段不是字符串",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"uuid":"123","args":{}}}`),
			desc: "调用请求缺少name字段",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"\r\n  \t","uuid":"123","args":{}}}`),
			desc: "name字段去除空格后为空",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"cat/qs","args":{}}}`),
			desc: "调用请求缺少uuid字段",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"cat/qs","uuid":"","args":{}}}`),
			desc: "uuid字段为空",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"cat/qs","uuid":"  \t","args":{}}}`),
			desc: "uuid字段去除空格后为空",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"cat/qs","uuid":"123"}}`),
			desc: "调用请求缺少args字段",
		},

		{
			msg:  []byte(`{"type":"call","payload":{"name":"cat/qs","uuid":"123","args":null}}`),
			desc: "args字段为null",
		},
	}

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnCall := new(mockCallReqHandler)
		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)

		server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
			"group": "A",
			"id":    "#1",
		})

		require.Nil(t, err)

		conn := newConn(server, mockedConn, WithClosedHandler(mockOnClose))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()
		mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		server.dealConn(conn)

		mockedConn.AssertExpectations(t)
		mockOnCall.AssertExpectations(t)
		mockOnClose.AssertExpectations(t)
	}
}

// TestDealResponseMsg 测试响应报文的处理逻辑
func TestDealResponseMsg(t *testing.T) {

	closedErr := errors.New("connection closed for: EOF")
	const uuidStr = `12345`

	type TestCase struct {
		msg      []byte          // 测试报文数据
		wantErr  error           // 期望的响应错误
		wantResp message.RawResp // 期望的响应数据
		desc     string          // 用例描述
	}

	testCases := []TestCase{
		{
			msg:      []byte(`{"type":"response","payload":[]}`),
			wantErr:  closedErr,
			wantResp: message.RawResp{},
			desc:     "payload不是对象",
		},

		{
			msg:      []byte(`{"type":"response","payload":{}}`),
			wantErr:  closedErr,
			wantResp: message.RawResp{},
			desc:     "payload缺失字段",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"   ","response":{}}}`),
			wantErr:  closedErr,
			wantResp: message.RawResp{},
			desc:     "uuid字段为空",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"12345","response":null}}`),
			wantErr:  closedErr,
			wantResp: message.RawResp{},
			desc:     "resp字段为null",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"not existed","response":{}}}`),
			wantErr:  closedErr,
			wantResp: message.RawResp{},
			desc:     "响应报文无对应的等待器",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"12345","response":{}}}`),
			wantErr:  nil,
			wantResp: message.RawResp{},
			desc:     "error字段缺失,响应为空",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"12345","error":"   ","response":{}}}`),
			wantErr:  nil,
			wantResp: message.RawResp{},
			desc:     "error字段为空字符串,响应为空",
		},

		{
			msg:      []byte(`{"type":"response","payload":{"uuid":"12345","error":"  arg \"a\": missing ","response":{}}}`),
			wantErr:  errors.New(`arg "a": missing`),
			wantResp: message.RawResp{},
			desc:     "error字段为有效错误,响应为空",
		},

		{
			msg:     []byte(`{"type":"response","payload":{"uuid":"12345","error":" ","response":{"res":true,"time":100}}}`),
			wantErr: nil,
			wantResp: message.RawResp{
				"res":  []byte(`true`),
				"time": []byte(`100`),
			},
			desc: "无错误信息的响应",
		},
	}

	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})
	require.Nil(t, err)

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)

		waiter := &RespWaiter{
			got: make(chan struct{}),
		}

		conn := newConn(server, mockedConn, WithClosedHandler(mockOnClose))
		conn.respWaiters = map[string]*RespWaiter{
			uuidStr: waiter,
		}

		// NOTE: 接收完响应报文就关闭连接,
		// NOTE: 如果没有收到正确的响应报文,等待器则会由于连接关闭而唤醒
		mockOnClose.On("OnClosed", io.EOF.Error()).Once()
		mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		server.dealConn(conn)

		select {
		case <-waiter.got:
			assert.EqualValues(t, test.wantErr, waiter.err, test.desc)
			assert.EqualValues(t, test.wantResp, waiter.resp, test.desc)
		default:
			assert.Fail(t, "等待器未唤醒", test.desc)
		}

		assert.Len(t, conn.respWaiters, 0, "响应等待器已清空")

		mockedConn.AssertExpectations(t)
		mockOnClose.AssertExpectations(t)
	}

	assert.Len(t, server.allConn, 0, "管理的连接必须为空")
}

// TestDealMetaInfoMsg 测试元信息报文处理逻辑
func TestDealMetaInfoMsg(t *testing.T) {

	type TestCase struct {
		msg     []byte // 测试报文数据
		wantErr error  // 期望的元信息错误
		desc    string // 用例描述
	}

	testCases := []TestCase{
		{
			msg:     nil,
			wantErr: errors.New("connection closed for: EOF"),
			desc:    "未收到元信息连接就关闭",
		},

		{
			msg:     []byte(`{"type":"meta-info","payload":123}`),
			wantErr: errors.New("root: NOT an object"),
			desc:    "元信息不为对象",
		},

		{
			msg:     []byte(`{"type":"meta-info","payload":{}}`),
			wantErr: errors.New("root: name NOT exist"),
			desc:    "name字段不存在",
		},

		{
			msg:     []byte(`{"type":"meta-info","payload":{"name":"tpqs"}}`),
			wantErr: errors.New("root: description NOT exist"),
			desc:    "description字段不存在",
		},
	}

	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	})
	require.Nil(t, err)

	for _, test := range testCases {
		fmt.Println(test.desc)

		mockOnClose := new(mockCloseHandler)
		mockedConn := new(mockConn)

		conn := newConn(server, mockedConn, WithClosedHandler(mockOnClose))

		mockOnClose.On("OnClosed", io.EOF.Error()).Once()
		if test.msg != nil {
			mockedConn.On("ReadMsg").Return(test.msg, nil).Once()
		}
		mockedConn.On("ReadMsg").Return([]byte(nil), io.EOF).Once()
		mockedConn.On("Close").Return(errors.New("already closed")).Once()

		server.dealConn(conn)

		select {
		case <-conn.metaGotCh:
		default:
			assert.Fail(t, "未唤醒元信息等待", test.desc)
		}

		assert.EqualValues(t, test.wantErr, conn.peerMetaErr, "期望的对端元信息错误")

		mockedConn.AssertExpectations(t)
		mockOnClose.AssertExpectations(t)
	}

}

// TestConnection_SubState 测试发送状态订阅报文
func TestConnection_SubState(t *testing.T) {
	type TestCase struct {
		states  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			states:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil",
		},

		{
			states:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil---发送失败",
		},

		{
			states:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":[]}`),
			desc:    "传入的states为空",
		},

		{
			states:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":[]}`),
			desc:    "传入的states为空---发送失败",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.SubState(test.states)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}

}

// TestConnection_AddSubState 测试发送添加状态订阅报文
func TestConnection_AddSubState(t *testing.T) {
	type TestCase struct {
		states  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			states:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil",
		},

		{
			states:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil---发送失败",
		},

		{
			states:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":[]}`),
			desc:    "传入的states为空",
		},

		{
			states:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":[]}`),
			desc:    "传入的states为空---发送失败",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.AddSubState(test.states)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

// TestConnection_CancelSubState 测试发送取消状态订阅报文
func TestConnection_CancelSubState(t *testing.T) {
	type TestCase struct {
		states  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			states:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil",
		},

		{
			states:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "传入的states为nil---发送失败",
		},

		{
			states:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "传入的states为空",
		},

		{
			states:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "传入的states为空---发送失败",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空",
		},

		{
			states:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":["A/a","B/b"]}`),
			desc:    "传入的states不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.CancelSubState(test.states)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

// TestConnection_CancelAllSubState 测试发送取消所有状态订阅报文
func TestConnection_CancelAllSubState(t *testing.T) {
	type TestCase struct {
		err     error  // 连接应答返回的错误信息
		wantMsg []byte // 连接期望发送的数据
		desc    string // 用例描述
	}

	testCases := []TestCase{
		{
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "发送成功",
		},

		{
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-state","payload":[]}`),
			desc:    "发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.CancelAllSubState()

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

// TestConnection_SubEvent 测试发送事件订阅报文
func TestConnection_SubEvent(t *testing.T) {
	type TestCase struct {
		events  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			events:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil",
		},

		{
			events:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil---发送失败",
		},

		{
			events:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":[]}`),
			desc:    "传入的events为空",
		},

		{
			events:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":[]}`),
			desc:    "传入的events为空---发送失败",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"set-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.SubEvent(test.events)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}

}

// TestConnection_AddSubEvent 测试发送添加事件订阅报文
func TestConnection_AddSubEvent(t *testing.T) {
	type TestCase struct {
		events  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			events:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil",
		},

		{
			events:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil---发送失败",
		},

		{
			events:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":[]}`),
			desc:    "传入的events为空",
		},

		{
			events:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":[]}`),
			desc:    "传入的events为空---发送失败",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"add-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.AddSubEvent(test.events)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

// TestConnection_CancelSubEvent 测试发送取消事件订阅报文
func TestConnection_CancelSubEvent(t *testing.T) {
	type TestCase struct {
		events  []string // 输入的状态列表
		err     error    // 连接应答返回的错误信息
		wantMsg []byte   // 连接期望发送的数据
		desc    string   // 用例描述
	}

	testCases := []TestCase{
		{
			events:  nil,
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil",
		},

		{
			events:  nil,
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "传入的events为nil---发送失败",
		},

		{
			events:  []string{},
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "传入的events为空",
		},

		{
			events:  []string{},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "传入的events为空---发送失败",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空",
		},

		{
			events:  []string{"A/a", "B/b"},
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":["A/a","B/b"]}`),
			desc:    "传入的events不为空---发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.CancelSubEvent(test.events)

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

// TestConnection_CancelAllSubEvent 测试发送取消所有状态订阅报文
func TestConnection_CancelAllSubEvent(t *testing.T) {
	type TestCase struct {
		err     error  // 连接应答返回的错误信息
		wantMsg []byte // 连接期望发送的数据
		desc    string // 用例描述
	}

	testCases := []TestCase{
		{
			err:     nil,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "发送成功",
		},

		{
			err:     io.EOF,
			wantMsg: []byte(`{"type":"remove-subscribe-event","payload":[]}`),
			desc:    "发送失败",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)

		mockedConn.On("WriteMsg", test.wantMsg).Return(test.err)

		gotErr := conn.CancelAllSubEvent()

		assert.EqualValues(t, test.err, gotErr, test.desc)

		mockedConn.AssertExpectations(t)
	}
}

func (s *StateEventSuite) TestDialTcp() {
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
