package server

import (
	jsoniter "github.com/json-iterator/go"
	"proxy/message"
)

type modelItem struct {
	ModelName string              `json:"modelName"`
	Addr      string              `json:"addr"`
	SubStates []string            `json:"subStates"`
	SubEvents []string            `json:"subEvents"`
	Meta      jsoniter.RawMessage `json:"meta"`
}

type queryOnlineReq struct {
	ModelName string
	ResChan   chan bool
}

type querySubRes struct {
	SubList []string `json:"subList"`
	Got     bool     `json:"got"`
}

type querySubStateReq struct {
	ModelName string
	ResChan   chan querySubRes
}

func (s *Server) dealProxyCall(call message.CallMessage, conn connection) {

	resp := message.Resp{}
	err := ""
	switch call.Method {
	case "GetAllModel":
		resp, err = s.GetAllModel()
	case "ModelIsOnline":
		resp, err = s.ModelIsOnline(call.Args)
	case "GetSubState":

	case "GetSubEvent":

	case "GetAllModelMeta":

	case "GetModelMeta":

	default:

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
	select {
	case s.queryAllModel <- resChan:
	case <-s.done:
		resp = message.Resp{}
		err = "proxy have quit"
		return
	}
	items := <-resChan
	resp = message.Resp{
		"modelList": items,
	}
	return
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

	select {
	case s.queryOnlineReq <- req:
	case <-s.done:
		return message.Resp{}, "proxy have quit"
	}

	return message.Resp{
		"isOnline": <-req.ResChan,
	}, ""
}

func (s *Server) GetSubState(Args map[string]jsoniter.RawMessage) (message.Resp, string) {
	var modelName string
	data, seen := Args["modelName"]
	if !seen {
		return message.Resp{}, "missing field \"modelName\" in args"
	}
	if err := jsoniter.Unmarshal(data, &modelName); err != nil {
		return message.Resp{}, err.Error()
	}

	req := querySubStateReq{
		ModelName: modelName,
		ResChan:   make(chan querySubRes, 1),
	}

	select {
	case s.querySubStateReq <- req:
	case <-s.done:
		return message.Resp{}, "proxy have quit"
	}

	res := <-req.ResChan

	return message.Resp{
		"subList": res.SubList,
		"got":     res.Got,
	}, ""
}
