package server

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"proxy/message"
	"time"
)

var upgrader = websocket.Upgrader{
	// 允许跨域访问
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	addConnChan    chan *model                         // 添加链路通道
	removeConnChan chan *model                         // 删除链路通道
	subStateChan   chan message.SubStateOrEventMessage // 订阅状态通道
	subEventChan   chan message.SubStateOrEventMessage // 订阅事件通道
	stateChan      chan message.StateOrEventMessage    // 状态报文通道
	eventChan      chan message.StateOrEventMessage    // 事件报文通道
	callChan       chan message.CallMessage            // 调用报文通道
	respChan       chan message.ResponseMessage        // 响应报文通道
	queryAllModel  chan chan []modelItem               // 查询在线模型通道
	queryModel     chan queryModelReq                  // 查询指定模型通道
	queryOnline    chan queryOnlineReq                 // 查询模型是否在线通道
	querySubState  chan querySubReq                    // 查询模型的状态订阅关系
	querySubEvent  chan querySubReq                    // 查询模型的事件订阅关系
}

func New() *Server {
	s := &Server{
		addConnChan:    make(chan *model),
		removeConnChan: make(chan *model),
		subStateChan:   make(chan message.SubStateOrEventMessage),
		subEventChan:   make(chan message.SubStateOrEventMessage),
		stateChan:      make(chan message.StateOrEventMessage),
		eventChan:      make(chan message.StateOrEventMessage),
		callChan:       make(chan message.CallMessage),
		respChan:       make(chan message.ResponseMessage),
		queryAllModel:  make(chan chan []modelItem),
		queryModel:     make(chan queryModelReq),
		queryOnline:    make(chan queryOnlineReq),
		querySubState:  make(chan querySubReq),
		querySubEvent:  make(chan querySubReq),
	}
	go s.run()
	return s
}

type connection struct {
	*model
	outCalls  map[string]struct{} // 自己发送的所有调用请求的UUID
	inCalls   map[string]struct{} // 所有发给自己的调用请求的UUID
	pubStates map[string]struct{} // 状态发布表, 用于记录哪些状态可以发送到链路上
	pubEvents map[string]struct{} // 事件发布表, 用于记录哪些事件可以发送到链路上
}

func (s *Server) ListenServeTCP(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		rawConn, err := l.Accept()
		if err != nil {
			return err
		}

		go s.addModelConnection(NewTcpConn(rawConn))
	}
}

func (s *Server) ListenServerWebSocket(addr string) error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		s.addModelConnection(NewWebSocketConn(conn))
	})
	return http.ListenAndServe(addr, nil)
}

func (s *Server) run() {
	// 所有连接
	connections := make(map[string]connection)
	// 等待响应的所有连接，uuid -> 发送调用请求的物模型名称
	respWaiters := make(map[string]string)
	for {
		select {
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
			s.onCall(call, connections, respWaiters)
		case resp := <-s.respChan:
			onResp(connections, resp, respWaiters)
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
			s.onAddConn(connections, m)
		case m := <-s.removeConnChan:
			s.onRemoveConn(connections, m, respWaiters)
		case resChan := <-s.queryAllModel:
			onQueryAllModel(connections, resChan)
		case queryModel := <-s.queryModel:
			onQueryModel(connections, queryModel)
		case isOnlineReq := <-s.queryOnline:
			_, seen := connections[isOnlineReq.ModelName]
			isOnlineReq.ResChan <- seen
		case querySubState := <-s.querySubState:
			onQuerySub(connections, querySubState, true)
		case querySubEvent := <-s.querySubEvent:
			onQuerySub(connections, querySubEvent, false)
		}
	}
}

func (s *Server) onCall(call message.CallMessage,
	connections map[string]connection,
	respWaiters map[string]string) {
	if call.Model == "proxy" {
		// 调用代理的方法
		go s.dealProxyCall(call, connections[call.Source])
		return
	}

	conn, seen := connections[call.Model]
	if !seen {
		// 期望调用的物模型不存在，直接返回错误响应
		errStr := fmt.Sprintf("model %q NOT exist", call.Model)
		resp := make(map[string]interface{})
		connections[call.Source].writeChan <- message.NewResponseFullData(call.UUID, errStr, resp)
		return
	}

	// 转发调用请求
	conn.writeChan <- call.FullData

	// 记录调用请求
	respWaiters[call.UUID] = call.Source
	conn.inCalls[call.UUID] = struct{}{}
	connections[call.Source].outCalls[call.UUID] = struct{}{}
}

func onResp(connections map[string]connection, resp message.ResponseMessage,
	respWaiters map[string]string) {
	// 不是在编的物模型连接发送的调用请求不响应
	if srcConn, seen := connections[resp.Source]; !seen {
		return
	} else {
		delete(srcConn.inCalls, resp.UUID)
	}
	// 响应无调用请求
	if _, seen := respWaiters[resp.UUID]; !seen {
		return
	}
	// 转发调用请求, 清空调用记录，必须判断等待调用请求的连接是否还在线
	if destConn, seen := connections[respWaiters[resp.UUID]]; seen {
		destConn.writeChan <- resp.FullData
		delete(destConn.outCalls, resp.UUID)
	}
	// 删除调用记录
	delete(respWaiters, resp.UUID)
}

func (s *Server) onAddConn(connections map[string]connection, m *model) {
	// 模型名称重复，直接关闭连接
	if _, repeat := connections[m.MetaInfo.Name]; repeat {
		go s.pushRepeatModelNameEvent(m)
		return
	}
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

	// 推送上线事件
	go s.pushOnlineOrOfflineEvent(m.MetaInfo.Name, m.RemoteAddr().String(), true)

	// 添加链路, 并通知已添加
	connections[m.MetaInfo.Name] = conn
	m.setAdded()
}

func (s *Server) onRemoveConn(connections map[string]connection, m *model,
	respWaiters map[string]string) {
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

		// 删除链路
		delete(connections, m.MetaInfo.Name)

		// 推送下线事件
		go s.pushOnlineOrOfflineEvent(m.MetaInfo.Name, m.RemoteAddr().String(), false)
	}

	// NOTE: 在此处quitWriter, 不会导致由于连接writer协程提前退出而导致的死锁
	// NOTE: 因为只有调用了quitWriter之后，writer协程才会退出
	m.quitWriter()
}

func onQueryAllModel(connections map[string]connection, resChan chan []modelItem) {
	items := make([]modelItem, 0, len(connections))
	for modelName, conn := range connections {
		states := make([]string, 0, len(conn.pubStates))
		events := make([]string, 0, len(conn.pubEvents))
		for state := range conn.pubStates {
			states = append(states, state)
		}
		for event := range conn.pubEvents {
			events = append(events, event)
		}
		items = append(items, modelItem{
			ModelName: modelName,
			Addr:      conn.model.RemoteAddr().String(),
			SubStates: states,
			SubEvents: events,
			MetaInfo:  conn.MetaInfo.FullData,
		})
	}
	resChan <- items
}

func onQueryModel(connections map[string]connection, queryModel queryModelReq) {
	info := modelItem{
		ModelName: "none",
		Addr:      "",
		SubStates: make([]string, 0),
		SubEvents: make([]string, 0),
		MetaInfo:  []byte(noneMetaJSON),
	}
	conn, seen := connections[queryModel.ModelName]
	if seen {
		info.ModelName = conn.MetaInfo.Name
		info.SubStates = make([]string, 0, len(conn.pubStates))
		for state := range conn.pubStates {
			info.SubStates = append(info.SubStates, state)
		}
		info.SubEvents = make([]string, 0, len(conn.pubEvents))
		for state := range conn.pubEvents {
			info.SubEvents = append(info.SubEvents, state)
		}
		info.Addr = conn.RemoteAddr().String()
		info.MetaInfo = conn.MetaInfo.FullData
	}
	queryModel.ResChan <- queryModelRes{
		ModelInfo: info,
		Got:       seen,
	}
}

func onQuerySub(connections map[string]connection, querySubState querySubReq, isState bool) {
	subList := make([]string, 0)
	conn, seen := connections[querySubState.ModelName]
	if seen {
		var subMap map[string]struct{}
		if isState {
			subMap = conn.pubStates
		} else {
			subMap = conn.pubEvents
		}
		subList = make([]string, 0, len(subMap))
		for pub := range subMap {
			subList = append(subList, pub)
		}
	}
	querySubState.ResChan <- querySubRes{
		SubList: subList,
		Got:     seen,
	}
}

func (s *Server) addModelConnection(conn ModelConn) {
	ans := &model{
		ModelConn:      conn,
		removeConnCh:   s.removeConnChan,
		stateBroadcast: s.stateChan,
		eventBroadcast: s.eventChan,
		callChan:       s.callChan,
		respChan:       s.respChan,
		subStateChan:   s.subStateChan,
		subEventChan:   s.subEventChan,
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

	// 元信息校验不通过则不添加, 并退出
	if err := ans.MetaInfo.Check(); err != nil {
		s.pushMetaCheckErrorEvent(err, ans)
		_ = ans.Close()
		return
	}

	// 添加链路
	s.addConnChan <- ans
}
