package model

import (
	"github.com/gorilla/websocket"
	"goModel/message"
	"goModel/meta"
	"goModel/rawConn"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
)

var upgrader = websocket.Upgrader{
	// 允许跨域访问
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ServiceTCPAddr struct {
	Name string // 模型名称
	IP   string // 模型服务IP地址
	Port int    // 模型服务端口号
}

// CallRequestFunc 为调用请求回调函数, 参数name为调用的方法名, 参数args为调用参数
type CallRequestFunc func(name string, args message.RawArgs) message.Resp

type Model struct {
	meta           *meta.Meta               // 元信息
	callReqHandler CallRequestFunc          // 调用请求处理函数
	connLock       sync.RWMutex             // 保护 allConn
	allConn        map[*Connection]struct{} // 所有连接
}

func NewEmptyModel() *Model {
	return &Model{
		meta:           meta.NewEmptyMeta(),
		callReqHandler: nil,
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
	parsed, err := meta.Parse(buff, tmpl)

	return &Model{
		meta:           parsed,
		callReqHandler: reqHandler,
	}, err
}

func (m *Model) Meta() *meta.Meta {
	return m.meta
}

func (m *Model) ListenServeTCP(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			return err
		}

		go m.dealConn(newConn(m, rawConn.NewTcpConn(conn), nil, nil))
	}
}

func (m *Model) ListenServeWebSocket(addr string) error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}

		m.dealConn(newConn(m, rawConn.NewWebSocketConn(conn), nil, nil))
	})
	return http.ListenAndServe(addr, nil)
}

func (m *Model) PushState(name string, data interface{}, verify bool) error {
	// 首先验证推送数据是否符合物模型元信息
	if verify {
		if err := m.meta.VerifyState(name, data); err != nil {
			return err
		}
	}

	// 全状态名 = 模型名/状态名
	fullName := strings.Join([]string{
		m.meta.Name,
		name,
	}, "/")

	// 向所有链路推送
	m.connLock.RLock()
	defer m.connLock.RUnlock()
	for conn := range m.allConn {
		conn.sendState(fullName, data)
	}

	return nil
}

func (m *Model) PushEvent(name string, args message.Args, verify bool) error {
	// 首先验证推送事件参数据是否符合物模型元信息
	if verify {
		if err := m.meta.VerifyEvent(name, args); err != nil {
			return err
		}
	}

	// 全事件名 = 模型名/事件名
	fullName := strings.Join([]string{
		m.meta.Name,
		name,
	}, "/")

	// 向所有链路推送
	m.connLock.RLock()
	defer m.connLock.RUnlock()
	for conn := range m.allConn {
		conn.sendEvent(fullName, args)
	}

	return nil
}

func (m *Model) dealConn(conn *Connection) {
	// 添加链接
	m.addConn(conn)

	// 处理接收
	conn.dealReceive()

	// 删除链接
	m.removeConn(conn)
}

func (m *Model) addConn(conn *Connection) {
	m.connLock.Lock()
	defer m.connLock.Unlock()
	m.allConn[conn] = struct{}{}
}

func (m *Model) removeConn(conn *Connection) {
	m.connLock.Lock()
	defer m.connLock.Unlock()
	delete(m.allConn, conn)
}
