package model

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/object-model/goModel/message"
	"github.com/object-model/goModel/meta"
	"github.com/object-model/goModel/rawConn"
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

// CallRequestHandler 调用请求处理接口
type CallRequestHandler interface {
	OnCallReq(name string, args message.RawArgs) message.Resp
}

// CallRequestFunc 为调用请求回调函数, 参数name为调用的方法名, 参数args为调用参数
// 函数返回值为调用请求的返回值.
type CallRequestFunc func(name string, args message.RawArgs) message.Resp

func (c CallRequestFunc) OnCallReq(name string, args message.RawArgs) message.Resp {
	return c(name, args)
}

// Model 表示物模型, 提供了元信息查询、状态和事件发布、与其他物模型建立连接、运行TCP服务和WebSocket服务功能.
// 若物模型的元信息包含方法, 并通过 WithCallReqHandler 或 WithCallReqFunc 注册了有效的调用请求回调,
// 在收到有效的调用请求报文时, 物模型将自动触发调用请求回调.
type Model struct {
	meta           *meta.Meta               // 元信息
	connLock       sync.RWMutex             // 保护 allConn
	allConn        map[*Connection]struct{} // 所有连接
	verifyResp     bool                     // 是否校验 callReqHandler 返回的响应返回值
	callReqHandler CallRequestHandler       // 调用请求处理函数
}

// ModelOption 为物模型创建选项
type ModelOption func(*Model)

// WithCallReqHandler 配置物模型的调用请求回调处理
func WithCallReqHandler(onCall CallRequestHandler) ModelOption {
	return func(model *Model) {
		if onCall != nil {
			model.callReqHandler = onCall
		}
	}
}

// WithCallReqFunc 配置物模型的调用请求回调函数对象
func WithCallReqFunc(onCall CallRequestFunc) ModelOption {
	return func(model *Model) {
		if onCall != nil {
			model.callReqHandler = onCall
		}
	}
}

// WithVerifyResp 开启物模型的响应校验选项
func WithVerifyResp() ModelOption {
	return func(model *Model) {
		model.verifyResp = true
	}
}

// NewEmptyModel 创建一个状态、事件、方法都为空的物模型.
func NewEmptyModel() *Model {
	return New(meta.NewEmptyMeta())
}

// LoadFromFile 从文件file中加载元信息, 设置元信息模板参数为tmpl, 并利用加载的元信息和配置参数opts创建物模型
// 返回创建的物模型和错误信息.
// 如果加载失败, LoadFromFile 会返回由 NewEmptyModel() 创建的空物模型和错误信息, LoadFromFile 不会返回值为nil的物模型.
func LoadFromFile(file string, tmpl meta.TemplateParam, opts ...ModelOption) (*Model, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return NewEmptyModel(), err
	}

	return LoadFromBuff(content, tmpl, opts...)
}

// LoadFromBuff 从缓存buff中加载元信息, 设置元信息模板参数为tmpl, 并利用加载的元信息和配置参数opts创建物模型
// 返回创建的物模型和错误信息.
// 如果加载失败, LoadFromBuff 会返回由 NewEmptyModel() 创建的空物模型和错误信息, LoadFromBuff 不会返回值为nil的物模型.
func LoadFromBuff(buff []byte, tmpl meta.TemplateParam, opts ...ModelOption) (*Model, error) {
	parsed, err := meta.Parse(buff, tmpl)

	return New(parsed, opts...), err
}

// New 根据参数opts创建元信息为meta的物模型并返回这个新创建的物模型.
func New(meta *meta.Meta, opts ...ModelOption) *Model {
	ans := &Model{
		meta:    meta,
		allConn: make(map[*Connection]struct{}),
	}

	for _, opt := range opts {
		opt(ans)
	}

	return ans
}

// Meta 返回物模型m所加载的元信息.
func (m *Model) Meta() *meta.Meta {
	return m.meta
}

// ListenServeTCP 开启对地址addr的监听, 并等待其他客户端物模型与m建立TCP连接.
// 所有建立的TCP连接自动开启 keep-alive 选项. ListenServeTCP 总是返回不为nil的错误信息.
//
// 客户端物模型可以同过 Dial("tcp@addr", opts...) 或者 DialTcp(addr, opts...) 与m建立连接.
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

		go m.dealConn(newConn(m, rawConn.NewTcpConn(conn, true)))
	}
}

// ListenServeWebSocket 在地址addr上开启http服务, 并等待其他客户端物模型与m建立WebSocket连接.
// 对于每个建立的WebSocket连接, m都会定时发送PING报文, 如果客户端未及时回复PONG报文, m将主动断开连接.
// ListenServeWebSocket 总是返回不为nil的错误信息.
//
// 客户端物模型可以同过 Dial("ws@addr", opts...) 或者 DialWebSocket("ws://addr", opts...) 与m建立连接.
func (m *Model) ListenServeWebSocket(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}

		m.dealConn(newConn(m, rawConn.NewWebSocketConn(conn, true)))
	})
	return http.ListenAndServe(addr, mux)
}

// PushState 推送名称为name, 数据为data的状态, m的所有连接只要是订阅了该状态, 都会收到该状态报文,
// 参数verify表示是否根据m的元信息校验状态数据, 若校验不通过返回错误信息, 其他情况都返回nil.
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

// PushEvent 推送名称为name, 参数为args的事件, m的所有连接只要是订阅了该事件, 都会收到该事件报文,
// 参数verify表示是否根据m的元信息校验事件参数, 若校验不通过返回错误信息, 其他情况都返回nil.
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

// Dial 根据连接配置opts使物模型m与地址为addr的服务端物模型建立连接, 返回所建立的连接和错误信息,
// 若建立连接成功, 返回的错误信息为nil, 若失败, 返回的连接为nil.
//
// 参数addr的有效格式为：network@ip:port
// 例如:
// 		tcp@localhost:8080
// 		tcp@192.168.1.51:http
// 		 ws@192.168.1.51:9090
// 协议network决定采用何种协议与服务端物模型建立连接:
// 		tcp: 使用TCP协议与服务端物模型建立连接, 等同于调用 DialTcp("ip:port", opts...)
// 		 ws: 使用WebSocket协议与服务端建立连接, 等同于调用 DialWebSocket("ws://ip:port", opts...)
func (m *Model) Dial(addr string, opts ...ConnOption) (*Connection, error) {
	i := strings.Index(addr, "@")
	if i == -1 {
		return nil, fmt.Errorf("%q missing @", addr)
	}

	network := addr[:i]
	_addr_ := addr[i+1:]

	switch network {
	case "ws":
		return m.DialWebSocket(network+"://"+_addr_, opts...)
	case "tcp":
		return m.DialTcp(_addr_, opts...)
	}

	return nil, fmt.Errorf("network %q is NOT supported", network)
}

// DialTcp 根据连接配置opts使物模型m与地址为addr的服务端物模型建立TCP连接, 返回所建立的连接和错误信息.
//
// 参数addr的有效格式为: ip:port
// 例如:
// 		localhost:8080
//		192.168.1.51:http
// 		192.168.1.51:9090
func (m *Model) DialTcp(addr string, opts ...ConnOption) (*Connection, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	raw, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}

	ans := newConn(m, rawConn.NewTcpConn(raw, false), opts...)
	go m.dealConn(ans)

	return ans, nil
}

// DialWebSocket 根据连接配opts使物模型m与地址为addr的服务端物模型建立WebSocket连接, 返回所建立的连接和错误信息.
//
// 参数addr的有效格式为: ws://ip:port
// 例如:
// 		ws://192.168.1.51:8080
// 		ws://localhost:8080
func (m *Model) DialWebSocket(addr string, opts ...ConnOption) (*Connection, error) {
	raw, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}

	ans := newConn(m, rawConn.NewWebSocketConn(raw, false), opts...)
	go m.dealConn(ans)

	return ans, nil
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
