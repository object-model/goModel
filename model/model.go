package model

import (
	"github.com/gorilla/websocket"
	"goModel/message"
	"goModel/meta"
	"goModel/rawConn"
	"io/ioutil"
	"net"
	"net/http"
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
type CallRequestFunc func(name string, args message.RawArgs)

type Model struct {
	meta           *meta.Meta
	callReqHandler CallRequestFunc
	connLock       sync.RWMutex // 保护 allConn
	allConn        map[*Connection]struct{}
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

		go m.dealConn(rawConn.NewTcpConn(conn))
	}
}

func (m *Model) ListenServeWebSocket(addr string) error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}

		m.dealConn(rawConn.NewWebSocketConn(conn))
	})
	return http.ListenAndServe(addr, nil)
}

func (m *Model) dealConn(raw rawConn.RawConn) {
	if raw == nil {
		return
	}

	conn := &Connection{
		raw: raw,
		m:   m,
	}

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
