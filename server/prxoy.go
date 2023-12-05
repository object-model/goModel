package server

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"proxy/message"
	"time"
)

type modelItem struct {
	ModelName string              `json:"modelName"`
	Addr      string              `json:"addr"`
	SubStates []string            `json:"subStates"`
	SubEvents []string            `json:"subEvents"`
	MetaInfo  jsoniter.RawMessage `json:"metaInfo"`
}

type queryModelRes struct {
	ModelInfo modelItem `json:"modelInfo"`
	Got       bool      `json:"got"`
}

type queryModelReq struct {
	ModelName string
	ResChan   chan queryModelRes
}

type queryOnlineReq struct {
	ModelName string
	ResChan   chan bool
}

type querySubRes struct {
	SubList []string `json:"subList"`
	Got     bool     `json:"got"`
}

type querySubReq struct {
	ModelName string
	ResChan   chan querySubRes
}

func (s *Server) dealProxyCall(call callMessage, conn connection) {
	resp := message.Resp{}
	errStr := ""
	switch call.Method {
	case "GetAllModel":
		resp, errStr = s.getAllModel()
	case "GetModel":
		resp, errStr = s.getModel(call.Args)
	case "ModelIsOnline":
		resp, errStr = s.modelIsOnline(call.Args)
	case "GetSubState":
		resp, errStr = s.getSubList(call.Args, s.querySubState)
	case "GetSubEvent":
		resp, errStr = s.getSubList(call.Args, s.querySubEvent)
	default:
		errStr = fmt.Sprintf("NO method %q in proxy", call.Method)
	}

	// 发送响应
	select {
	case conn.writeChan <- message.Must(message.EncodeRespMsg(call.UUID, errStr, resp)):
	case <-conn.writerQuit:
		return
	}
}

func (s *Server) getAllModel() (resp message.Resp, err string) {
	resChan := make(chan []modelItem, 1)
	s.queryAllModel <- resChan
	items := <-resChan
	resp = message.Resp{
		"modelList": items,
	}
	return
}

func (s *Server) getModel(Args map[string]jsoniter.RawMessage) (resp message.Resp, err string) {
	var modelName string
	data, seen := Args["modelName"]
	if !seen {
		return message.Resp{}, "missing field \"modelName\" in args"
	}
	if err := jsoniter.Unmarshal(data, &modelName); err != nil {
		return message.Resp{}, err.Error()
	}

	req := queryModelReq{
		ModelName: modelName,
		ResChan:   make(chan queryModelRes, 1),
	}

	s.queryModel <- req
	res := <-req.ResChan

	return message.Resp{
		"modelInfo": res.ModelInfo,
		"got":       res.Got,
	}, ""
}

func (s *Server) modelIsOnline(Args map[string]jsoniter.RawMessage) (message.Resp, string) {
	var modelName string
	data, seen := Args["modelName"]
	if !seen {
		return message.Resp{}, "missing field \"modelName\" in args"
	}
	if err := jsoniter.Unmarshal(data, &modelName); err != nil {
		return message.Resp{}, err.Error()
	}

	req := queryOnlineReq{
		ModelName: modelName,
		ResChan:   make(chan bool, 1),
	}
	s.queryOnline <- req

	return message.Resp{
		"isOnline": <-req.ResChan,
	}, ""
}

func (s *Server) getSubList(Args map[string]jsoniter.RawMessage, queryChan chan<- querySubReq) (message.Resp, string) {
	var modelName string
	data, seen := Args["modelName"]
	if !seen {
		return message.Resp{}, "missing field \"modelName\" in args"
	}
	if err := jsoniter.Unmarshal(data, &modelName); err != nil {
		return message.Resp{}, err.Error()
	}

	req := querySubReq{
		ModelName: modelName,
		ResChan:   make(chan querySubRes, 1),
	}

	queryChan <- req

	res := <-req.ResChan

	return message.Resp{
		"subList": res.SubList,
		"got":     res.Got,
	}, ""
}

func (s *Server) pushOnlineOrOfflineEvent(modelName string, addr string, online bool) {
	EventName := "proxy/offline"
	if online {
		EventName = "proxy/online"
	}

	fullData := message.Must(message.EncodeEventMsg(EventName, message.Args{
		"modelName": modelName,
		"addr":      addr,
	}))

	s.eventChan <- stateOrEventMessage{
		Source:   "proxy",
		Name:     EventName,
		FullData: fullData,
	}
}

func (s *Server) pushMetaCheckErrorEvent(checkErr error, m *model) {
	fullData := message.Must(message.EncodeEventMsg("metaCheckError", message.Args{
		"modelName": m.MetaInfo.Name,
		"addr":      m.RemoteAddr().String(),
		"error":     checkErr.Error(),
	}))

	event := stateOrEventMessage{
		Source:   "proxy",
		Name:     "metaCheckError",
		FullData: fullData,
	}

	// 无论m是否订阅metaCheckError事件都主动推送
	m.writeChan <- event.FullData

	// 正常推送事件
	s.eventChan <- event

	// NOTE: 延时关闭连接，尽量确保状态event能发送
	time.Sleep(time.Second)
}

func (s *Server) pushRepeatModelNameEvent(m *model) {
	fullData := message.Must(message.EncodeEventMsg("repeatModelNameError", message.Args{
		"modelName": m.MetaInfo.Name,
		"addr":      m.RemoteAddr().String(),
	}))

	event := stateOrEventMessage{
		Source:   "proxy",
		Name:     "repeatModelNameError",
		FullData: fullData,
	}

	// 无论m是否订阅repeatModelNameError事件都主动推送
	m.writeChan <- event.FullData

	// 正常推送事件
	s.eventChan <- event

	// NOTE: 延时关闭连接，尽量确保状态event能发送
	time.Sleep(time.Second)

	_ = m.Close()
}
