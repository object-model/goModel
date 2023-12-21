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
	"sync"
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

// TestCallRequestFunc_OnCallReq 测试调用请求回调函数的逻辑
func TestCallRequestFunc_OnCallReq(t *testing.T) {
	wantName := "QS"
	wantArgs := message.RawArgs{
		"angle": []byte(`90`),
		"speed": []byte(`superFast`),
	}
	wantResp := message.Resp{
		"res":  true,
		"msg":  "执行成功",
		"time": uint(10000),
		"code": 0,
	}

	var onCall CallRequestFunc = func(name string, args message.RawArgs) message.Resp {
		assert.Equal(t, wantName, name, "name参数")
		assert.Equal(t, wantArgs, args, "args参数")
		return wantResp
	}

	got := onCall.OnCallReq(wantName, wantArgs)

	assert.Equal(t, wantResp, got, "返回值")
}

// TestStateFunc_OnState 测试状态回调函数的逻辑
func TestStateFunc_OnState(t *testing.T) {
	wantModelName := "A/car/#1/tpqs"
	wantStateName := "tpqsInfo"
	wantData := []byte(`{"qsState":"erecting","hpSwitch":false,"qsAngle":90}`)

	var onState StateFunc = func(modelName string, stateName string, data []byte) {
		assert.Equal(t, wantModelName, modelName, "modelName参数")
		assert.Equal(t, wantStateName, stateName, "stateName参数")
		assert.Equal(t, wantData, data, "data参数")
	}

	onState.OnState(wantModelName, wantStateName, wantData)
}

// TestEventFunc_OnEvent 测试事件回调函数的逻辑
func TestEventFunc_OnEvent(t *testing.T) {
	wantModelName := "A/car/#1/tpqs"
	wantEventName := "qsMotorOverCur"
	wantArgs := message.RawArgs{}

	var onEvent EventFunc = func(modelName string, eventName string, args message.RawArgs) {
		assert.Equal(t, wantModelName, modelName, "modelName参数")
		assert.Equal(t, wantEventName, eventName, "eventName参数")
		assert.Equal(t, wantArgs, args, "args参数")
	}

	onEvent.OnEvent(wantModelName, wantEventName, wantArgs)
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

// TestWithCallReqFunc 测试配置物模型调用请求回调函数
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

// TestWithStateBuffSize 测试配置连接状态缓存区大小
func TestWithStateBuffSize(t *testing.T) {
	conn := &Connection{}

	WithStateBuffSize(100)(conn)

	assert.Equal(t, 100, cap(conn.statesChan), "配置状态缓存大小")
}

// TestWithEventBuffSize 测试配置连接事件缓存区大小
func TestWithEventBuffSize(t *testing.T) {
	conn := &Connection{}

	WithEventBuffSize(100)(conn)

	assert.Equal(t, 100, cap(conn.eventsChan), "配置状态缓存大小")
}

// TestWithStateFunc 测试配置连接状态回调处理函数
func TestWithStateFunc(t *testing.T) {
	conn := &Connection{}

	onState := func(modelName string, stateName string, data []byte) {}

	WithStateFunc(onState)(conn)

	assert.Equal(t, reflect.ValueOf(onState).Pointer(),
		reflect.ValueOf(conn.stateHandler).Pointer(),
		"配置状态回调处理函数")
}

// TestWithEventFunc 测试配置连接事件回调处理函数
func TestWithEventFunc(t *testing.T) {
	conn := &Connection{}

	onEvent := func(modelName string, eventName string, args message.RawArgs) {}

	WithEventFunc(onEvent)(conn)

	assert.Equal(t, reflect.ValueOf(onEvent).Pointer(),
		reflect.ValueOf(conn.eventHandler).Pointer(),
		"配置事件回调处理函数")
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

// TestConnection_Invoke 测试异步调用接口
func TestConnection_Invoke(t *testing.T) {
	type TestCase struct {
		name    string       // 调用的方法名
		args    message.Args // 调用参数
		uid     string       // uuid生成的字符串
		wantErr error        // 期望返回的错误信息
		sendMsg bool         // 连接是否发送数据
		wantMsg []byte       // 连接期望发送的数据
		desc    string       // 用例描述
	}

	uidPitcher := func(test TestCase) func() string {
		return func() string {
			return test.uid
		}
	}

	testCases := []TestCase{
		{
			name: "m/qs",
			args: message.Args{
				"a": func() {},
			},
			wantErr: errors.New("encode call args failed"),
			desc:    "调用参数无法编码---包含函数",
		},

		{
			name: "m/qs",
			args: message.Args{
				"a": make(chan int),
			},
			wantErr: errors.New("encode call args failed"),
			desc:    "调用参数无法编码---包含管道",
		},

		{
			name:    "m/qs",
			args:    nil,
			uid:     "123",
			sendMsg: true,
			wantMsg: []byte(`{"type":"call","payload":{"name":"m/qs","uuid":"123","args":{}}}`),
			wantErr: io.EOF,
			desc:    "发送失败",
		},

		{
			name:    "m/qs",
			args:    nil,
			uid:     "123",
			sendMsg: true,
			wantMsg: []byte(`{"type":"call","payload":{"name":"m/qs","uuid":"123","args":{}}}`),
			wantErr: nil,
			desc:    "发送成功---参数为空",
		},

		{
			name: "m/qs",
			args: message.Args{
				"cmd":   "QS",
				"speed": "fast",
			},
			uid:     "45678",
			sendMsg: true,
			wantMsg: []byte(`{"type":"call","payload":{"name":"m/qs","uuid":"45678","args":{"cmd":"QS","speed":"fast"}}}`),
			wantErr: nil,
			desc:    "发送成功---参数不为空",
		},
	}

	for _, test := range testCases {
		mockedConn := new(mockConn)
		conn := newConn(NewEmptyModel(), mockedConn)
		conn.uidCreator = uidPitcher(test)

		if test.sendMsg {
			mockedConn.On("WriteMsg", test.wantMsg).Return(test.wantErr)
		}

		resp, gotErr := conn.Invoke(test.name, test.args)

		assert.EqualValues(t, test.wantErr, gotErr, test.desc)

		if test.wantErr == nil {
			assert.Contains(t, conn.respWaiters, test.uid, test.desc)
			assert.Equal(t, conn.respWaiters[test.uid], resp, test.desc)
		} else {
			// 返回错误的情况下, 等待器一定为nil
			assert.Nil(t, resp, test.desc)
		}

		mockedConn.AssertExpectations(t)
	}
}

// 同步调用示例
type callSample struct {
	args    message.Args    // 客户端的调用参数
	rawArgs message.RawArgs // 服务端收到的调用参数
	resp    message.Resp    // 服务端回调返回的响应值
	rawResp message.RawResp // 客户端收到的响应值
	err     error           // 客户端调用错误信息
}

// CallSuite 为同步调用测试套件
type CallSuite struct {
	suite.Suite
	server     *Model              // 服务端物模型
	mockOnCall *mockCallReqHandler // 调用请求模拟
	calls      []callSample        // 同步调用示例
	tcpAddr    string              // tcp地址
	wsAddr     string              // ws地址
	wg         sync.WaitGroup      // 等待客户端完成信号
}

func (c *CallSuite) SetupSuite() {
	c.tcpAddr = "localhost:65432"
	c.wsAddr = "localhost:61888"

	c.mockOnCall = new(mockCallReqHandler)
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReqHandler(c.mockOnCall))
	require.Nil(c.T(), err)

	c.server = server

	c.calls = []callSample{
		{
			args: message.Args{
				"angle": 90,
				"speed": "fast",
			},
			rawArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(90000),
				"code": 0,
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`90000`),
				"code": []byte(`0`),
			},
			err: nil,
		},

		{
			args: message.Args{
				"angle": 45,
				"speed": "middle",
			},
			rawArgs: message.RawArgs{
				"angle": []byte(`45`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
				"code": 0,
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`45000`),
				"code": []byte(`0`),
			},
			err: nil,
		},

		{
			args: message.Args{
				"angle": 45,
				"speed": "superFast",
			},
			rawArgs: message.RawArgs{
				"angle": []byte(`45`),
				"speed": []byte(`"superFast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`45000`),
			},
			err: errors.New("response \"code\": missing"),
		},

		{
			args: message.Args{
				"angle": 60,
				"speed": "superFast",
			},
			rawArgs: message.RawArgs{
				"angle": []byte(`60`),
				"speed": []byte(`"superFast"`),
			},
			resp: message.Resp{
				"res":  false,
				"msg":  "执行失败",
				"time": uint(45000),
				"code": 4,
			},
			rawResp: message.RawResp{
				"res":  []byte(`false`),
				"msg":  []byte(`"执行失败"`),
				"time": []byte(`45000`),
				"code": []byte(`4`),
			},
			err: errors.New("response \"code\": 4 NOT in option"),
		},
	}

	for _, call := range c.calls {
		c.mockOnCall.
			On("OnCallReq", "QS", call.rawArgs).
			Return(call.resp).
			Times(2)
	}

	go func() {
		fmt.Printf("listen tcp@%s\n", c.tcpAddr)
		err := server.ListenServeTCP(c.tcpAddr)
		require.NotNil(c.T(), err)
	}()

	go func() {
		fmt.Printf("listen ws@%s\n", c.wsAddr)
		err := server.ListenServeWebSocket(c.wsAddr)
		require.NotNil(c.T(), err)
	}()

	c.wg.Add(2)
}

func (c *CallSuite) client(conn *Connection) {
	// 查询对端元信息
	metaInfo, err := conn.GetPeerMeta()
	require.Nil(c.T(), err, "首次获取对端元信息")

	assert.Equal(c.T(), "A/car/#1/tpqs", metaInfo.Name, "对端元信息---模型名")
	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/tpqsInfo",
		"A/car/#1/tpqs/powerInfo",
		"A/car/#1/tpqs/gear",
		"A/car/#1/tpqs/QSCount",
	}, metaInfo.AllStates(), "对端元信息---事件名")

	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/qsMotorOverCur",
		"A/car/#1/tpqs/qsAction",
	}, metaInfo.AllEvents(), "对端元信息---事件名")

	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/QS",
	}, metaInfo.AllMethods(), "对端元信息---方法名")

	// 二次查询元信息
	metaInfo2, err := conn.GetPeerMeta()
	require.Nil(c.T(), err, "第二次获取对端元信息")
	assert.Equal(c.T(), metaInfo, metaInfo2, "两次获取的元信息要相同")

	// 同步调用方法
	for _, call := range c.calls {
		resp, err := conn.Call("A/car/#1/tpqs/QS", call.args)
		assert.EqualValues(c.T(), call.err, err)
		assert.EqualValues(c.T(), call.rawResp, resp)
	}
}

func (c *CallSuite) TestServerSide() {
	c.T().Parallel()

	// 等待所有客户端都运行完了
	c.wg.Wait()

	// NOTE: 最后再断言调用请求回调
	c.mockOnCall.AssertExpectations(c.T())
}

func (c *CallSuite) TestTCPClient() {
	defer c.wg.Done()

	c.T().Parallel()

	conn, err := NewEmptyModel().DialTcp(c.tcpAddr)

	require.Nil(c.T(), err)
	require.NotNil(c.T(), conn)

	c.client(conn)
}

func (c *CallSuite) TestWSClient() {
	defer c.wg.Done()

	c.T().Parallel()

	conn, err := NewEmptyModel().DialWebSocket("ws://" + c.wsAddr)

	require.Nil(c.T(), err)
	require.NotNil(c.T(), conn)

	c.client(conn)
}

// 测试调用请求报文发送失败时Call的返回逻辑
func (c *CallSuite) TestSendCallFailed() {
	mockedConn := new(mockConn)

	conn := newConn(c.server, mockedConn)
	conn.uidCreator = func() string {
		return "123"
	}

	callMsg := `{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123","args":{}}}`
	mockedConn.On("WriteMsg", []byte(callMsg)).Return(io.EOF).Once()

	resp, err := conn.Call("A/car/#1/tpqs/QS", nil)
	assert.Equal(c.T(), message.RawResp{}, resp, "发送调用请求失败时---返回响应为空")
	assert.Equal(c.T(), io.EOF, err, "发送调用请求失败时---返回的错误信息")

	mockedConn.AssertExpectations(c.T())
}

// 测试员信息查询报文发送失败时GetPeerMeta的返回逻辑
func (c *CallSuite) TestSendQueryMetaFailed() {
	mockedConn := new(mockConn)

	conn := newConn(c.server, mockedConn)

	queryMsg := `{"type":"query-meta","payload":null}`
	mockedConn.On("WriteMsg", []byte(queryMsg)).Return(io.EOF).Once()

	peerMeta, err := conn.GetPeerMeta()

	assert.Equal(c.T(), conn.peerMeta, peerMeta, "发送元信息查询报文失败时---返回的元信息")
	assert.Equal(c.T(), io.EOF, err, "发送元信息查询报文失败时---返回的错误信息")

	mockedConn.AssertExpectations(c.T())
}

// TestCall 测试真实环境下同步调用
func TestCall(t *testing.T) {
	suite.Run(t, new(CallSuite))
}

// 同步加超时调用示例
// NOTE: 不同用例的参数应该不相同, 否则会导致测试不同的奇怪行为!!!
// NOTE: 原因是模拟的接口mockCallReqHandler无法识别出相同参数的调用的区别
type callForSample struct {
	args     message.Args    // 客户端的调用参数
	callIsOn bool            // 服务端回调函数是否触发
	rawArgs  message.RawArgs // 服务端收到的调用参数
	resp     message.Resp    // 服务端回调返回的响应值
	exeTime  time.Duration   // 服务端回调执行时间
	rawResp  message.RawResp // 客户端收到的响应值
	waitTime time.Duration   // 客户端等待时间
	err      error           // 客户端调用错误信息
	desc     string          // 用例描述
}

// CallForSuite 为同步+超时调用测试套件
type CallForSuite struct {
	suite.Suite
	server     *Model              // 服务端物模型
	mockOnCall *mockCallReqHandler // 调用请求模拟
	calls      []callForSample     // 同步+超时调用示例
	tcpAddr    string              // tcp地址
	wsAddr     string              // ws地址
	wg         sync.WaitGroup      // 等待客户端完成信号
}

func (c *CallForSuite) SetupSuite() {
	c.tcpAddr = "localhost:60000"
	c.wsAddr = "localhost:60001"

	c.mockOnCall = new(mockCallReqHandler)
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReqHandler(c.mockOnCall))
	require.Nil(c.T(), err)

	c.server = server

	c.calls = []callForSample{
		{
			args: message.Args{
				"angle": 90,
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("arg \"speed\": missing"),
			desc:     "调用参数不符合元信息---调用参数缺失",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "unknown",
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("arg \"speed\": \"unknown\" NOT in option"),
			desc:     "调用参数不符合元信息---调用参数不再可选项范围中",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "fast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(90000),
				"code": 0,
			},
			exeTime: 0,
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`90000`),
				"code": []byte(`0`),
			},
			waitTime: time.Second,
			err:      nil,
			desc:     "执行成功---等待时间大于执行时间",
		},

		{
			args: message.Args{
				"angle": 45,
				"speed": "middle",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`45`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
				"code": 0,
			},
			exeTime:  time.Second * 2,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("timeout"),
			desc:     "执行成功---等待超时小于执行时间",
		},

		{
			args: message.Args{
				"angle": 45,
				"speed": "superFast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`45`),
				"speed": []byte(`"superFast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
			},
			exeTime: time.Second,
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`45000`),
			},
			waitTime: time.Second * 2,
			err:      errors.New("response \"code\": missing"),
			desc:     "回调函数返回值不符合元信息---等待时间大于执行时间",
		},

		{
			args: message.Args{
				"angle": 60,
				"speed": "superFast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`60`),
				"speed": []byte(`"superFast"`),
			},
			resp: message.Resp{
				"res":  false,
				"msg":  "执行失败",
				"time": uint(45000),
				"code": 4,
			},
			exeTime:  time.Second * 2,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("timeout"),
			desc:     "回调函数返回值不符合元信息---等待时间小于执行时间",
		},
	}

	for _, call := range c.calls {
		if call.callIsOn {
			c.mockOnCall.
				On("OnCallReq", "QS", call.rawArgs).
				After(call.exeTime).
				Return(call.resp).
				Times(2)
		}
	}

	go func() {
		fmt.Printf("listen tcp@%s\n", c.tcpAddr)
		err := server.ListenServeTCP(c.tcpAddr)
		require.NotNil(c.T(), err)
	}()

	go func() {
		fmt.Printf("listen ws@%s\n", c.wsAddr)
		err := server.ListenServeWebSocket(c.wsAddr)
		require.NotNil(c.T(), err)
	}()

	c.wg.Add(2)
}

func (c *CallForSuite) client(conn *Connection) {
	// 查询对端元信息
	metaInfo, err := conn.GetPeerMeta()
	require.Nil(c.T(), err, "首次获取对端元信息")

	assert.Equal(c.T(), "A/car/#1/tpqs", metaInfo.Name, "对端元信息---模型名")
	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/tpqsInfo",
		"A/car/#1/tpqs/powerInfo",
		"A/car/#1/tpqs/gear",
		"A/car/#1/tpqs/QSCount",
	}, metaInfo.AllStates(), "对端元信息---事件名")

	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/qsMotorOverCur",
		"A/car/#1/tpqs/qsAction",
	}, metaInfo.AllEvents(), "对端元信息---事件名")

	assert.Equal(c.T(), []string{
		"A/car/#1/tpqs/QS",
	}, metaInfo.AllMethods(), "对端元信息---方法名")

	// 二次查询元信息
	metaInfo2, err := conn.GetPeerMeta()
	require.Nil(c.T(), err, "第二次获取对端元信息")
	assert.Equal(c.T(), metaInfo, metaInfo2, "两次获取的元信息要相同")

	// 同步调用方法
	for _, call := range c.calls {
		resp, err := conn.CallFor("A/car/#1/tpqs/QS", call.args, call.waitTime)
		assert.EqualValues(c.T(), call.err, err, call.desc)
		assert.EqualValues(c.T(), call.rawResp, resp, call.desc)
	}
}

func (c *CallForSuite) TestServerSide() {
	c.T().Parallel()

	// 等待所有客户端都运行完了
	c.wg.Wait()

	// NOTE: 最后再断言调用请求回调
	c.mockOnCall.AssertExpectations(c.T())
}

func (c *CallForSuite) TestTCPClient() {
	defer c.wg.Done()

	c.T().Parallel()

	conn, err := NewEmptyModel().DialTcp(c.tcpAddr)

	require.Nil(c.T(), err)
	require.NotNil(c.T(), conn)

	c.client(conn)
}

func (c *CallForSuite) TestWSClient() {
	defer c.wg.Done()

	c.T().Parallel()

	conn, err := NewEmptyModel().DialWebSocket("ws://" + c.wsAddr)

	require.Nil(c.T(), err)
	require.NotNil(c.T(), conn)

	c.client(conn)
}

// 测试调用请求报文发送失败时CallFor的返回逻辑
func (c *CallForSuite) TestSendCallFailed() {
	mockedConn := new(mockConn)

	conn := newConn(c.server, mockedConn)
	conn.uidCreator = func() string {
		return "123"
	}

	callMsg := `{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123","args":{}}}`
	mockedConn.On("WriteMsg", []byte(callMsg)).Return(io.EOF).Once()

	resp, err := conn.CallFor("A/car/#1/tpqs/QS", nil, time.Second)
	assert.Equal(c.T(), message.RawResp{}, resp, "发送调用请求失败时---返回响应为空")
	assert.Equal(c.T(), io.EOF, err, "发送调用请求失败时---返回的错误信息")

	mockedConn.AssertExpectations(c.T())
}

// TestCallFor 测试真实环境下同步+超时调用
func TestCallFor(t *testing.T) {
	suite.Run(t, new(CallForSuite))
}

// 异步+回调调用示例
type invokeCallbackSample struct {
	args     message.Args    // 客户端的调用参数
	callIsOn bool            // 服务端回调函数是否触发
	rawArgs  message.RawArgs // 服务端收到的调用参数
	resp     message.Resp    // 服务端回调返回的响应值
	rawResp  message.RawResp // 客户端回调收到的响应值
	err      error           // 客户端回调收到错误信息
	desc     string          // 用例描述
}

// InvokeCallbackSuite 为异步+回调方式调用测试套件
type InvokeCallbackSuite struct {
	suite.Suite
	server     *Model                 // 服务端物模型
	mockOnCall *mockCallReqHandler    // 调用请求模拟
	calls      []invokeCallbackSample // 异步+回调调用示例
	tcpAddr    string                 // tcp地址
	wsAddr     string                 // ws地址
	wg         sync.WaitGroup         // 等待客户端完成信号
	timeOut    time.Duration          // 所有响应回调必须在该时间内完成执行
}

func (callbackSuite *InvokeCallbackSuite) SetupSuite() {
	callbackSuite.tcpAddr = "localhost:50000"
	callbackSuite.wsAddr = "localhost:50001"

	callbackSuite.mockOnCall = new(mockCallReqHandler)
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReqHandler(callbackSuite.mockOnCall))
	require.Nil(callbackSuite.T(), err)

	callbackSuite.server = server

	callbackSuite.calls = []invokeCallbackSample{
		{
			args: message.Args{
				"angle": 90,
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			err:      errors.New("arg \"speed\": missing"),
			desc:     "调用参数不符合元信息---调用参数缺失",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "unknown",
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			err:      errors.New("arg \"speed\": \"unknown\" NOT in option"),
			desc:     "调用参数不符合元信息---调用参数不再可选项范围中",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "fast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(90000),
				"code": 0,
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`90000`),
				"code": []byte(`0`),
			},
			err:  nil,
			desc: "执行成功",
		},

		{
			args: message.Args{
				"angle": 45,
				"speed": "superFast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`45`),
				"speed": []byte(`"superFast"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`45000`),
			},
			err:  errors.New("response \"code\": missing"),
			desc: "回调函数返回值不符合元信息---参数缺失",
		},
	}

	for _, call := range callbackSuite.calls {
		if call.callIsOn {
			callbackSuite.mockOnCall.
				On("OnCallReq", "QS", call.rawArgs).
				Return(call.resp).
				Times(2)
		}
	}

	go func() {
		fmt.Printf("listen tcp@%s\n", callbackSuite.tcpAddr)
		err := server.ListenServeTCP(callbackSuite.tcpAddr)
		require.NotNil(callbackSuite.T(), err)
	}()

	go func() {
		fmt.Printf("listen ws@%s\n", callbackSuite.wsAddr)
		err := server.ListenServeWebSocket(callbackSuite.wsAddr)
		require.NotNil(callbackSuite.T(), err)
	}()

	callbackSuite.wg.Add(2)

	callbackSuite.timeOut = time.Second
}

func (callbackSuite *InvokeCallbackSuite) client(conn *Connection) {
	called := make(chan struct{}, len(callbackSuite.calls))

	// 回调函数创建器
	creatRespFunc := func(call invokeCallbackSample) RespFunc {
		return func(resp message.RawResp, err error) {
			assert.Equal(callbackSuite.T(), call.rawResp, resp, "调用结果回调函数收到的响应", call.desc)
			assert.Equal(callbackSuite.T(), call.err, err, "调用结果回调函数收到的错误信息", call.desc)
			called <- struct{}{}
		}
	}

	// 异步+回调调用方法
	for _, call := range callbackSuite.calls {
		err := conn.InvokeByCallback("A/car/#1/tpqs/QS", call.args, creatRespFunc(call))
		assert.Nil(callbackSuite.T(), err, call.desc)
	}

	// 确保所有回调函数执行了再退出
	timer := time.NewTimer(callbackSuite.timeOut)
	for i := 0; i < len(callbackSuite.calls); i++ {
		select {
		case <-timer.C:
			callbackSuite.Fail("所有异步调用结果回调在规定时间内未执行完成")
		case <-called:
		}
	}
}

func (callbackSuite *InvokeCallbackSuite) TestServerSide() {
	callbackSuite.T().Parallel()

	// 等待所有客户端都运行完了
	callbackSuite.wg.Wait()

	// NOTE: 最后再断言调用请求回调
	callbackSuite.mockOnCall.AssertExpectations(callbackSuite.T())
}

func (callbackSuite *InvokeCallbackSuite) TestTCPClient() {
	defer callbackSuite.wg.Done()

	callbackSuite.T().Parallel()

	conn, err := NewEmptyModel().DialTcp(callbackSuite.tcpAddr)

	require.Nil(callbackSuite.T(), err)
	require.NotNil(callbackSuite.T(), conn)

	callbackSuite.client(conn)
}

func (callbackSuite *InvokeCallbackSuite) TestWSClient() {
	defer callbackSuite.wg.Done()

	callbackSuite.T().Parallel()

	conn, err := NewEmptyModel().DialWebSocket("ws://" + callbackSuite.wsAddr)

	require.Nil(callbackSuite.T(), err)
	require.NotNil(callbackSuite.T(), conn)

	callbackSuite.client(conn)
}

// 测试调用请求报文发送失败时 InvokeByCallback 的返回逻辑
func (callbackSuite *InvokeCallbackSuite) TestSendCallFailed() {
	mockedConn := new(mockConn)

	conn := newConn(callbackSuite.server, mockedConn)
	conn.uidCreator = func() string {
		return "123"
	}

	callMsg := `{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123","args":{}}}`
	mockedConn.On("WriteMsg", []byte(callMsg)).Return(io.EOF).Once()

	err := conn.InvokeByCallback("A/car/#1/tpqs/QS", nil, func(resp message.RawResp, err error) {
		callbackSuite.Fail("发送调用请求报文失败时回调函数不应当被触发")
	})
	assert.Equal(callbackSuite.T(), io.EOF, err, "发送调用请求失败时---返回的错误信息")

	mockedConn.AssertExpectations(callbackSuite.T())
}

// TestInvokeByCallback 测试真实环境下异步+回调调用
func TestInvokeByCallback(t *testing.T) {
	suite.Run(t, new(InvokeCallbackSuite))
}

// 异步+回调+超时 调用示例
// NOTE: 不同用例的参数应该不相同, 否则会导致测试不同的奇怪行为!!!
// NOTE: 原因是模拟的接口mockCallReqHandler无法识别出相同参数的调用的区别
type invokeForSample struct {
	args     message.Args    // 客户端的调用参数
	callIsOn bool            // 服务端回调函数是否触发
	rawArgs  message.RawArgs // 服务端收到的调用参数
	exeTime  time.Duration   // 服务端调用请求处理函数执行时间
	resp     message.Resp    // 服务端回调返回的响应值
	waitTime time.Duration   // 客户端等待时间
	rawResp  message.RawResp // 客户端回调收到的响应值
	err      error           // 客户端回调收到错误信息
	desc     string          // 用例描述
}

// InvokeForSuite 为异步+回调+超时方式调用测试套件
type InvokeForSuite struct {
	suite.Suite
	server     *Model              // 服务端物模型
	mockOnCall *mockCallReqHandler // 调用请求模拟
	calls      []invokeForSample   // 异步+回调+超时调用示例
	tcpAddr    string              // tcp地址
	wsAddr     string              // ws地址
	wg         sync.WaitGroup      // 等待客户端完成信号
}

func (invokeForSuite *InvokeForSuite) SetupSuite() {
	invokeForSuite.tcpAddr = "localhost:58888"
	invokeForSuite.wsAddr = "localhost:59999"

	invokeForSuite.mockOnCall = new(mockCallReqHandler)
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReqHandler(invokeForSuite.mockOnCall))
	require.Nil(invokeForSuite.T(), err)

	invokeForSuite.server = server

	invokeForSuite.calls = []invokeForSample{
		{
			args: message.Args{
				"angle": 90,
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("arg \"speed\": missing"),
			desc:     "调用参数不符合元信息---调用参数缺失",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "unknown",
			},
			callIsOn: false,
			rawResp:  message.RawResp{},
			waitTime: time.Second,
			err:      errors.New("arg \"speed\": \"unknown\" NOT in option"),
			desc:     "调用参数不符合元信息---调用参数不再可选项范围中",
		},

		{
			args: message.Args{
				"angle": 90,
				"speed": "fast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`90`),
				"speed": []byte(`"fast"`),
			},
			exeTime: 0,
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(90000),
				"code": 0,
			},
			waitTime: time.Second,
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`90000`),
				"code": []byte(`0`),
			},
			err:  nil,
			desc: "执行成功---等待时间大于执行时间",
		},

		{
			args: message.Args{
				"angle": 85,
				"speed": "fast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`85`),
				"speed": []byte(`"fast"`),
			},
			exeTime: time.Second * 2,
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(90000),
				"code": 0,
			},
			waitTime: time.Second,
			rawResp:  message.RawResp{},
			err:      errors.New("timeout"),
			desc:     "执行成功---等待时间小于执行时间",
		},

		{
			args: message.Args{
				"angle": 40,
				"speed": "superFast",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`40`),
				"speed": []byte(`"superFast"`),
			},
			exeTime: time.Second,
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(45000),
			},
			rawResp: message.RawResp{
				"res":  []byte(`true`),
				"msg":  []byte(`"执行成功"`),
				"time": []byte(`45000`),
			},
			waitTime: time.Second * 2,
			err:      errors.New("response \"code\": missing"),
			desc:     "回调函数返回值不符合元信息---等待时间大于执行时间",
		},

		{
			args: message.Args{
				"angle": 66,
				"speed": "middle",
			},
			callIsOn: true,
			rawArgs: message.RawArgs{
				"angle": []byte(`66`),
				"speed": []byte(`"middle"`),
			},
			resp: message.Resp{
				"res":  true,
				"msg":  "执行成功",
				"time": uint(50000),
			},
			exeTime:  time.Second * 2,
			rawResp:  message.RawResp{},
			waitTime: time.Second * 1,
			err:      errors.New("timeout"),
			desc:     "回调函数返回值不符合元信息---等待时间小于执行时间",
		},
	}

	for _, call := range invokeForSuite.calls {
		if call.callIsOn {
			invokeForSuite.mockOnCall.
				On("OnCallReq", "QS", call.rawArgs).
				After(call.exeTime).
				Return(call.resp).
				Times(2)
		}
	}

	go func() {
		fmt.Printf("listen tcp@%s\n", invokeForSuite.tcpAddr)
		err := server.ListenServeTCP(invokeForSuite.tcpAddr)
		require.NotNil(invokeForSuite.T(), err)
	}()

	go func() {
		fmt.Printf("listen ws@%s\n", invokeForSuite.wsAddr)
		err := server.ListenServeWebSocket(invokeForSuite.wsAddr)
		require.NotNil(invokeForSuite.T(), err)
	}()

	invokeForSuite.wg.Add(2)
}

func (invokeForSuite *InvokeForSuite) client(conn *Connection) {
	called := make(chan struct{}, len(invokeForSuite.calls))

	// 回调函数创建器
	creatRespFunc := func(call invokeForSample) RespFunc {
		return func(resp message.RawResp, err error) {
			assert.Equal(invokeForSuite.T(), call.rawResp, resp, "调用结果回调函数收到的响应", call.desc)
			assert.Equal(invokeForSuite.T(), call.err, err, "调用结果回调函数收到的错误信息", call.desc)
			called <- struct{}{}
		}
	}

	// 所有响应回调函数总的执行时间限制
	// = 所有调用请求的等待时间 waitTime 之和 + 1s
	var timeout time.Duration

	// 异步+回调+超时 调用方法
	for _, call := range invokeForSuite.calls {
		timeout += call.waitTime
		err := conn.InvokeFor("A/car/#1/tpqs/QS", call.args, creatRespFunc(call), call.waitTime)
		assert.Nil(invokeForSuite.T(), err, call.desc)
	}
	timeout += time.Second

	// 确保所有回调函数执行了再退出
	timer := time.NewTimer(timeout)
	for i := 0; i < len(invokeForSuite.calls); i++ {
		select {
		case <-timer.C:
			invokeForSuite.Fail("所有异步调用结果回调在规定时间内未执行完成")
		case <-called:
		}
	}
}

func (invokeForSuite *InvokeForSuite) TestServerSide() {
	invokeForSuite.T().Parallel()

	// 等待所有客户端都运行完了
	invokeForSuite.wg.Wait()

	// NOTE: 最后再断言调用请求回调
	invokeForSuite.mockOnCall.AssertExpectations(invokeForSuite.T())
}

func (invokeForSuite *InvokeForSuite) TestTCPClient() {
	defer invokeForSuite.wg.Done()

	invokeForSuite.T().Parallel()

	conn, err := NewEmptyModel().DialTcp(invokeForSuite.tcpAddr)

	require.Nil(invokeForSuite.T(), err)
	require.NotNil(invokeForSuite.T(), conn)

	invokeForSuite.client(conn)
}

func (invokeForSuite *InvokeForSuite) TestWSClient() {
	defer invokeForSuite.wg.Done()

	invokeForSuite.T().Parallel()

	conn, err := NewEmptyModel().DialWebSocket("ws://" + invokeForSuite.wsAddr)

	require.Nil(invokeForSuite.T(), err)
	require.NotNil(invokeForSuite.T(), conn)

	invokeForSuite.client(conn)
}

// 测试调用请求报文发送失败时 InvokeByCallback 的返回逻辑
func (invokeForSuite *InvokeForSuite) TestSendCallFailed() {
	mockedConn := new(mockConn)

	conn := newConn(invokeForSuite.server, mockedConn)
	conn.uidCreator = func() string {
		return "123"
	}

	callMsg := `{"type":"call","payload":{"name":"A/car/#1/tpqs/QS","uuid":"123","args":{}}}`
	mockedConn.On("WriteMsg", []byte(callMsg)).Return(io.EOF).Once()

	err := conn.InvokeFor("A/car/#1/tpqs/QS", nil, func(resp message.RawResp, err error) {
		invokeForSuite.Fail("发送调用请求报文失败时回调函数不应当被触发")
	}, time.Second)
	assert.Equal(invokeForSuite.T(), io.EOF, err, "发送调用请求失败时---返回的错误信息")

	mockedConn.AssertExpectations(invokeForSuite.T())
}

// TestInvokeFor 测试真实环境下异步+回调+超时的方式调用
func TestInvokeFor(t *testing.T) {
	suite.Run(t, new(InvokeForSuite))
}

// CallCloseSuite 为真实环境下远程调用后主动关闭连接场景下的测试套件
type CallCloseSuite struct {
	suite.Suite
	server     *Model              // 服务端物模型
	mockOnCall *mockCallReqHandler // 调用请求模拟
	tcpAddr    string              // tcp地址
	wsAddr     string              // ws地址
	args       message.Args        // 客户端的调用参数
	rawArgs    message.RawArgs     // 服务端收到的调用参数
	exeTime    time.Duration       // 方法执行事件
	waitNum    int                 // 并行等待的数量
	wg         sync.WaitGroup      // 等待客户端完成信号
}

func (closeSuite *CallCloseSuite) SetupSuite() {
	closeSuite.tcpAddr = "localhost:51888"
	closeSuite.wsAddr = "localhost:51999"
	closeSuite.exeTime = time.Second * 4
	closeSuite.args = message.Args{
		"angle": 90,
		"speed": "fast",
	}
	closeSuite.rawArgs = message.RawArgs{
		"angle": []byte(`90`),
		"speed": []byte(`"fast"`),
	}
	closeSuite.waitNum = 100

	closeSuite.mockOnCall = new(mockCallReqHandler)
	server, err := LoadFromFile("../meta/tpqs.json", meta.TemplateParam{
		"group": "A",
		"id":    "#1",
	}, WithVerifyResp(), WithCallReqHandler(closeSuite.mockOnCall))
	require.Nil(closeSuite.T(), err)

	closeSuite.server = server

	resp := message.Resp{
		"res":  true,
		"msg":  "执行成功",
		"time": uint(90000),
		"code": 0,
	}

	closeSuite.mockOnCall.
		On("OnCallReq", "QS", closeSuite.rawArgs).
		After(closeSuite.exeTime).
		Return(resp).
		Times(2)

	go func() {
		fmt.Printf("listen tcp@%s\n", closeSuite.tcpAddr)
		err := server.ListenServeTCP(closeSuite.tcpAddr)
		require.NotNil(closeSuite.T(), err)
	}()

	go func() {
		fmt.Printf("listen ws@%s\n", closeSuite.wsAddr)
		err := server.ListenServeWebSocket(closeSuite.wsAddr)
		require.NotNil(closeSuite.T(), err)
	}()

	closeSuite.wg.Add(2)
}

func (closeSuite *CallCloseSuite) client(conn *Connection) {
	waiter, err := conn.Invoke("A/car/#1/tpqs/QS", closeSuite.args)
	closeSuite.Nil(err, "连接必须建立成功")

	done := make(chan struct{}, closeSuite.waitNum*2)

	waitFunc := func(waiter *RespWaiter) {
		defer func() { done <- struct{}{} }()
		resp, gotErr := waiter.Wait()
		closeSuite.Require().Equal(errors.New("connection closed for: active close"), gotErr)
		closeSuite.Assert().Equal(message.RawResp{}, resp)
	}

	waitForFunc := func(waiter *RespWaiter) {
		defer func() { done <- struct{}{} }()
		resp, gotErr := waiter.WaitFor(closeSuite.exeTime + time.Second)

		closeSuite.Require().Equal(errors.New("connection closed for: active close"), gotErr)
		closeSuite.Assert().Equal(message.RawResp{}, resp)
	}

	// 开启多个协程无限等待一个响应
	var timeout time.Duration
	for i := 0; i < closeSuite.waitNum; i++ {
		go waitFunc(waiter)
		go waitForFunc(waiter)
		timeout += time.Second
	}

	// 方法执行到一半时间时关闭连接
	time.Sleep(closeSuite.exeTime / 2)
	_ = conn.Close()

	// 等待回调函数执行完毕
	for i := 0; i < closeSuite.waitNum*2; i++ {
		select {
		case <-done:
		}
	}
}

func (closeSuite *CallCloseSuite) TestServerSide() {
	closeSuite.T().Parallel()

	// 等待所有客户端都运行完了
	closeSuite.wg.Wait()

	// NOTE: 最后再断言调用请求回调
	closeSuite.mockOnCall.AssertExpectations(closeSuite.T())
}

func (closeSuite *CallCloseSuite) TestTCPClient() {
	defer closeSuite.wg.Done()

	closeSuite.T().Parallel()

	conn, err := NewEmptyModel().DialTcp(closeSuite.tcpAddr, WithClosedFunc(func(reason string) {
		closeSuite.Equal("active close", reason)
	}))

	require.Nil(closeSuite.T(), err)
	require.NotNil(closeSuite.T(), conn)

	closeSuite.client(conn)
}

func (closeSuite *CallCloseSuite) TestWSClient() {
	defer closeSuite.wg.Done()

	closeSuite.T().Parallel()

	conn, err := NewEmptyModel().DialWebSocket("ws://"+closeSuite.wsAddr, WithClosedFunc(func(reason string) {
		closeSuite.Equal("active close", reason)
	}))

	require.Nil(closeSuite.T(), err)
	require.NotNil(closeSuite.T(), conn)

	closeSuite.client(conn)
}

// TestCallClose 测试真实环境下远程调用后主动关闭连接的场景
func TestCallClose(t *testing.T) {
	suite.Run(t, new(CallCloseSuite))
}
