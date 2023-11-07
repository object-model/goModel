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
		resp, err = s.GetAllModel()
	case "GetModel":
		resp, err = s.GetModel(call.Args)
	case "ModelIsOnline":
		resp, err = s.ModelIsOnline(call.Args)
	case "GetSubState":
		resp, err = s.GetSubList(call.Args, s.querySubState)
	case "GetSubEvent":
		resp, err = s.GetSubList(call.Args, s.querySubEvent)
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

func (s *Server) GetAllModel() (resp message.Resp, err string) {
	resChan := make(chan []modelItem, 1)
	s.queryAllModel <- resChan
	items := <-resChan
	resp = message.Resp{
		"modelList": items,
	}
	return
}

func (s *Server) GetModel(Args map[string]jsoniter.RawMessage) (resp message.Resp, err string) {
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

func (s *Server) ModelIsOnline(Args map[string]jsoniter.RawMessage) (message.Resp, string) {
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

func (s *Server) GetSubList(Args map[string]jsoniter.RawMessage, queryChan chan<- querySubReq) (message.Resp, string) {
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
