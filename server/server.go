package server

import (
	"fmt"
	"net"
	"proxy/message"
	"time"
)

type Server struct {
	addConnChan    chan *ModelConnection
	removeConnChan chan *ModelConnection
	stateChan      chan message.StateOrEventMessage
	eventChan      chan message.StateOrEventMessage
	callChan       chan message.CallMessage
	respChan       chan message.ResponseMessage
	quitCh         chan struct{}
	done           chan struct{}
}

func New() *Server {
	return &Server{
		addConnChan:    make(chan *ModelConnection),
		removeConnChan: make(chan *ModelConnection),
		stateChan:      make(chan message.StateOrEventMessage),
		eventChan:      make(chan message.StateOrEventMessage),
		callChan:       make(chan message.CallMessage),
		respChan:       make(chan message.ResponseMessage),
		quitCh:         make(chan struct{}),
		done:           make(chan struct{}),
	}
}

func (s *Server) ListenServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go s.run()
	defer s.quit()

	for {
		rawConn, err := l.Accept()
		if err != nil {
			return err
		}

		go s.addModelConnection(rawConn)
	}
}

func (s *Server) run() {
	defer close(s.done)
	connections := make(map[string]*ModelConnection) // 所有连接
	respWaiters := make(map[string]string)           // 等待响应的所有连接，uuid -> 链路的物模型名称
	for {
		select {
		case <-s.quitCh:
			for _, conn := range connections {
				// NOTE: 关闭连接，使在Read的reader主动退出
				_ = conn.Close()
			}
			return
		case state := <-s.stateChan:
			// 不是在线的物模型连接发送的状态报文不转发
			if _, seen := connections[state.Source]; !seen {
				continue
			}
			for _, conn := range connections {
				conn.stateWriteChan <- state
			}
		case event := <-s.eventChan:
			// 不是在编的物模型连接发送的事件报文不转发
			if _, seen := connections[event.Source]; !seen {
				continue
			}
			for _, conn := range connections {
				conn.eventWriteChan <- event
			}
		case call := <-s.callChan:
			// 不是在编的物模型连接发送的调用请求不响应
			if _, seen := connections[call.Source]; !seen {
				continue
			}
			// 期望调用的物模型不存在
			if conn, seen := connections[call.Model]; !seen {
				errStr := fmt.Sprintf("model %q NOT exist", call.Model)
				resp := make(map[string]interface{})
				connections[call.Source].respWriteChan <- message.ResponseMessage{
					Source:   "proxy",
					UUID:     call.UUID,
					FullData: message.NewResponseFullData(call.UUID, errStr, resp),
				}
				continue
			} else {
				// 转发调用请求
				conn.callWriteChan <- call
				// 记录调用请求
				respWaiters[call.UUID] = call.Source
			}
		case resp := <-s.respChan:
			// 不是在编的物模型连接发送的调用请求不响应
			if _, seen := connections[resp.Source]; !seen {
				continue
			}
			// 响应无调用请求
			if _, seen := respWaiters[resp.UUID]; !seen {
				continue
			}
			// 转发调用请求, 必须判断等待调用请求的连接是否还在线
			if destConn, seen := connections[respWaiters[resp.UUID]]; seen {
				destConn.respWriteChan <- resp
			}

			// 删除调用记录
			delete(respWaiters, resp.UUID)
		case conn := <-s.addConnChan:
			// 订阅所有状态
			conn.remoteSubStateCh <- updateSubTableMsg{
				option: setSub,
				items:  conn.MetaInfo.AllStates(),
			}
			// 订阅所有事件
			conn.remoteSubEventCh <- updateSubTableMsg{
				option: setSub,
				items:  conn.MetaInfo.AllEvents(),
			}
			// 添加链路
			connections[conn.MetaInfo.Name] = conn
		case conn := <-s.removeConnChan:
			delete(connections, conn.MetaInfo.Name)
			// NOTE: 在此处quitWriter, 不会导致由于连接writer协程提前退出而导致的死锁
			// NOTE: 因为只有调用了quitWriter之后，writer协程才会退出
			conn.quitWriter()
		}
	}
}

func (s *Server) addModelConnection(conn net.Conn) {
	ans := &ModelConnection{
		Conn:               conn,
		localSubStateCh:    make(chan updateSubTableMsg),
		localSubEventCh:    make(chan updateSubTableMsg),
		remoteSubStateCh:   make(chan updateSubTableMsg),
		remoteSubEventCh:   make(chan updateSubTableMsg),
		querySubState:      make(chan chan []string),
		querySubEvent:      make(chan chan []string),
		removeConnCh:       s.removeConnChan,
		stateBroadcast:     s.stateChan,
		eventBroadcast:     s.eventChan,
		callChan:           s.callChan,
		respChan:           s.respChan,
		serverDone:         s.done,
		stateWriteChan:     make(chan message.StateOrEventMessage, 256),
		eventWriteChan:     make(chan message.StateOrEventMessage, 256),
		callWriteChan:      make(chan message.CallMessage, 256),
		respWriteChan:      make(chan message.ResponseMessage, 256),
		quit:               make(chan struct{}),
		queryProxyMetaChan: make(chan struct{}),
		queryMetaChan:      make(chan struct{}),
		metaGotChan:        make(chan struct{}),
	}

	go ans.writer()
	go ans.reader()

	// 发送查询元信息报文
	if err := ans.queryMeta(time.Second * 5); err != nil {
		// NOTE: 调用Close而不调用quitWriter
		// NOTE: 这样保证链路协程的退出顺序始终为：
		// NOTE: Close() -> reader退出 —> 向Server发出链路退出信号 ->
		// NOTE: 关闭链路quit通道 -> writer退出
		ans.Close()
		return
	}

	// TODO: 添加元信息校验，元信息校验不通过则不添加, 并退出
	// if ans.MetaInfo.Check() != nil {
	// 	ans.Close()
	// }

	// 添加链路
	s.addConnChan <- ans
}

func (s *Server) quit() {
	close(s.quitCh)
}
