package model

import (
	"goModel/message"
	"goModel/meta"
	"io/ioutil"
)

// CallRequestFunc 为调用请求回调函数, 参数name为调用的方法名, 参数args为调用参数
type CallRequestFunc func(name string, args message.RawArgs)

type Model struct {
	meta           *meta.Meta
	callReqHandler CallRequestFunc
}

func NewEmptyModel() *Model {
	return &Model{
		meta:           meta.NewEmptyMeta(),
		callReqHandler: func(string, message.RawArgs) {},
	}
}

func LoadFromFile(file string, tmpl meta.TemplateParam, reqHandler CallRequestFunc) (*Model, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return NewEmptyModel(), err
	}

	return LoadFromBuff(content, tmpl, reqHandler)
}

func LoadFromBuff(buff []byte, tmpl meta.TemplateParam, reqHandler CallRequestFunc) (*Model, error) {
	if reqHandler == nil {
		reqHandler = func(string, message.RawArgs) {}
	}

	parsed, err := meta.Parse(buff, tmpl)

	return &Model{
		meta:           parsed,
		callReqHandler: reqHandler,
	}, err
}

func (m *Model) Meta() *meta.Meta {
	return m.meta
}
