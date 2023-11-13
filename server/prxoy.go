package server

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"proxy/message"
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

func (s *Server) dealProxyCall(call message.CallMessage, conn connection) {
	resp := message.Resp{}
	err := ""
	switch call.Method {
	case "GetAllModel":
		resp, err = s.getAllModel()
	case "GetModel":
		resp, err = s.getModel(call.Args)
	case "ModelIsOnline":
		resp, err = s.modelIsOnline(call.Args)
	case "GetSubState":
		resp, err = s.getSubList(call.Args, s.querySubState)
	case "GetSubEvent":
		resp, err = s.getSubList(call.Args, s.querySubEvent)
	default:
		err = fmt.Sprintf("NO method %q in proxy", call.Method)
	}

	// 编码响应
	respData := message.NewResponseFullData(call.UUID, err, resp)

	// 发送响应
	select {
	case conn.writeChan <- respData:
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

	args := map[string]interface{}{
		"modelName": modelName,
		"addr":      addr,
	}

	eventPayload := message.EventPayload{
		Name: EventName,
		Args: args,
	}

	eventPayloadData, err := jsoniter.Marshal(eventPayload)
	if err != nil {
		return
	}

	msg := message.Message{
		Type:    "event",
		Payload: eventPayloadData,
	}

	fullData, err := jsoniter.Marshal(msg)
	if err != nil {
		return
	}

	s.eventChan <- message.StateOrEventMessage{
		Source:   "proxy",
		Name:     EventName,
		FullData: fullData,
	}
}
