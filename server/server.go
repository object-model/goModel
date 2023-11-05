package server

import (
	"fmt"
	"net"
	"proxy/message"
	"time"
)

type Server struct {
	addConnChan    chan *model
	removeConnChan chan *model
	subStateChan   chan message.SubStateOrEventMessage
	subEventChan   chan message.SubStateOrEventMessage
	stateChan      chan message.StateOrEventMessage
	eventChan      chan message.StateOrEventMessage
	callChan       chan message.CallMessage
	respChan       chan message.ResponseMessage
	quitCh         chan struct{}
	done           chan struct{}
}

func New() *Server {
	return &Server{
		addConnChan:    make(chan *model),
		removeConnChan: make(chan *model),
		subStateChan:   make(chan message.SubStateOrEventMessage),
		subEventChan:   make(chan message.SubStateOrEventMessage),
		stateChan:      make(chan message.StateOrEventMessage),
		eventChan:      make(chan message.StateOrEventMessage),
		callChan:       make(chan message.CallMessage),
		respChan:       make(chan message.ResponseMessage),
		quitCh:         make(chan struct{}),
		done:           make(chan struct{}),
	}
}

type connection struct {
	*model
	outCalls  map[string]struct{} // 自己发送的所有调用请求的UUID
	inCalls   map[string]struct{} // 所有发给自己的调用请求的UUID
	pubStates map[string]struct{} // 状态发布表, 用于记录哪些状态可以发送到链路上
	pubEvents map[string]struct{} // 事件发布表, 用于记录哪些事件可以发送到链路上
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
	// 所有连接
	connections := make(map[string]connection)
	// 等待响应的所有连接，uuid -> 发送调用请求的物模型名称
	respWaiters := make(map[string]string)
	for {
		select {
		case <-s.quitCh:
			for _, conn := range connections {
				// NOTE: 关闭连接，使在Read的reader主动退出
				_ = conn.Close()
			}
			return
		case state := <-s.stateChan:
			for _, conn := range connections {
				if _, want := conn.pubStates[state.Name]; want {
					conn.writeChan <- state.FullData
				}
			}
		case event := <-s.eventChan:
			for _, conn := range connections {
				if _, want := conn.pubEvents[event.Name]; want {
					conn.writeChan <- event.FullData
				}
			}
		case call := <-s.callChan:
			// 期望调用的物模型不存在
			if conn, seen := connections[call.Model]; !seen {
				errStr := fmt.Sprintf("model %q NOT exist", call.Model)
				resp := make(map[string]interface{})
				connections[call.Source].writeChan <- message.NewResponseFullData(call.UUID, errStr, resp)
				continue
			} else {
				// 转发调用请求
				conn.writeChan <- call.FullData

				// 记录调用请求
				respWaiters[call.UUID] = call.Source
				conn.inCalls[call.UUID] = struct{}{}
				connections[call.Source].outCalls[call.UUID] = struct{}{}
			}
		case resp := <-s.respChan:
			// 不是在编的物模型连接发送的调用请求不响应
			if srcConn, seen := connections[resp.Source]; !seen {
				continue
			} else {
				delete(srcConn.inCalls, resp.UUID)
			}
			// 响应无调用请求
			if _, seen := respWaiters[resp.UUID]; !seen {
				continue
			}
			// 转发调用请求, 清空调用记录，必须判断等待调用请求的连接是否还在线
			if destConn, seen := connections[respWaiters[resp.UUID]]; seen {
				destConn.writeChan <- resp.FullData
				delete(destConn.outCalls, resp.UUID)
			}
			// 删除调用记录
			delete(respWaiters, resp.UUID)
		case subStateReq := <-s.subStateChan:
			if conn, seen := connections[subStateReq.Source]; seen {
				conn.pubStates = message.UpdatePubTable(subStateReq, conn.pubStates)
				connections[subStateReq.Source] = conn
			}
		case subEventReq := <-s.subEventChan:
			if conn, seen := connections[subEventReq.Source]; seen {
				conn.pubEvents = message.UpdatePubTable(subEventReq, conn.pubEvents)
				connections[subEventReq.Source] = conn
			}
		case m := <-s.addConnChan:
			// 订阅所有状态
			data, _ := message.NewPubStateMessage(message.SetSub, m.MetaInfo.AllStates())
			m.writeChan <- data

			// 订阅所有事件
			data, _ = message.NewPubEventMessage(message.SetSub, m.MetaInfo.AllEvents())
			m.writeChan <- data

			conn := connection{
				model:     m,
				outCalls:  map[string]struct{}{},
				inCalls:   map[string]struct{}{},
				pubStates: map[string]struct{}{},
				pubEvents: map[string]struct{}{},
			}

			// 添加链路, 并通知已添加
			connections[m.MetaInfo.Name] = conn
			close(m.added)
		case m := <-s.removeConnChan:
			if conn, seen := connections[m.MetaInfo.Name]; seen {
				// 通知所有等待本连接响应报文的调用请求 可以不用等了
				errStr := fmt.Sprintf("model %q have quit", m.MetaInfo.Name)
				empty := make(map[string]interface{})
				for uuid := range conn.inCalls {
					if destConn, ok := connections[respWaiters[uuid]]; ok {
						destConn.writeChan <- message.NewResponseFullData(uuid, errStr, empty)
					}
				}

				// 清空本连接的等待的所有调用
				for uuid := range conn.outCalls {
					delete(respWaiters, uuid)
				}
			}
			// 删除链路
			delete(connections, m.MetaInfo.Name)

			// NOTE: 在此处quitWriter, 不会导致由于连接writer协程提前退出而导致的死锁
			// NOTE: 因为只有调用了quitWriter之后，writer协程才会退出
			m.quitWriter()
		}
	}
}

func (s *Server) addModelConnection(conn net.Conn) {
	ans := &model{
		Conn:           conn,
		removeConnCh:   s.removeConnChan,
		stateBroadcast: s.stateChan,
		eventBroadcast: s.eventChan,
		callChan:       s.callChan,
		respChan:       s.respChan,
		subStateChan:   s.subStateChan,
		subEventChan:   s.subEventChan,
		serverDone:     s.done,
		writeChan:      make(chan []byte, 256),
		writerQuit:     make(chan struct{}),
		readerQuit:     make(chan struct{}),
		added:          make(chan struct{}),
		metaGotChan:    make(chan struct{}),
	}

	go ans.writer()
	go ans.reader()

	// 发送查询元信息报文
	if err := ans.queryMeta(time.Second * 5); err != nil {
		// NOTE: 调用Close而不调用quitWriter
		// NOTE: 这样保证链路协程的退出顺序始终为：
		// NOTE: Close() -> reader退出 —> 向Server发出链路退出信号 ->
		// NOTE: 关闭链路writerQuit通道 -> writer退出
		_ = ans.Close()
		return
	}

	// TODO: 添加元信息校验，元信息校验不通过则不添加, 并退出
	// if ans.MetaInfo.Check() != nil {
	//    ans.Close()
	//    return
	// }

	// 添加链路
	s.addConnChan <- ans
}

func (s *Server) quit() {
	close(s.quitCh)
}
