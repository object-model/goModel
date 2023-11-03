package server

import (
	"net"
	"proxy/message"
	"time"
)

type Server struct {
	addConnChan    chan *ModelConnection
	removeConnChan chan *ModelConnection
	stateChan      chan message.StateOrEventMessage
	eventChan      chan message.StateOrEventMessage
	quitCh         chan struct{}
}

func New() *Server {
	return &Server{
		addConnChan:    make(chan *ModelConnection),
		removeConnChan: make(chan *ModelConnection),
		stateChan:      make(chan message.StateOrEventMessage),
		eventChan:      make(chan message.StateOrEventMessage),
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
	connections := make(map[string]*ModelConnection) // 所有连接

	for {
		select {
		case <-s.quitCh:
			return
		case state := <-s.stateChan:
			for _, conn := range connections {
				conn.stateWriteChan <- state
			}
		case event := <-s.eventChan:
			for _, conn := range connections {
				conn.eventWriteChan <- event
			}
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
			// NOTE: 在此处quitWriter, 不会导致由于连接writer协程体检退出而导致的死锁
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
		stateWriteChan:     make(chan message.StateOrEventMessage, 256),
		eventWriteChan:     make(chan message.StateOrEventMessage, 256),
		quit:               make(chan struct{}),
		queryProxyMetaChan: make(chan struct{}),
		queryMetaChan:      make(chan struct{}),
		metaGotChan:        make(chan struct{}),
	}

	go ans.writer()
	go ans.reader()

	// 发送查询元信息报文
	if err := ans.queryMeta(time.Second * 5); err != nil {
		ans.quitWriter()
	}

	// TODO: 添加元信息校验，元信息校验不通过则不添加, 并退出
	// if ans.MetaInfo.Check() != nil {
	// 	ans.quitWriter()
	// }

	// 添加链路
	s.addConnChan <- ans
}

func (s *Server) quit() {
	close(s.quitCh)
}
