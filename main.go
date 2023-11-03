package main

import (
	"fmt"
	"log"
	"net"
	"proxy/message"
	"time"
)

var (
	addConnChan    = make(chan *ModelConnection)
	removeConnChan = make(chan *ModelConnection)
	stateChan      = make(chan message.StateOrEventMessage, 256)
	eventChan      = make(chan message.StateOrEventMessage, 256)
)

func addModelConnection(rawConn net.Conn, removeConnCh chan *ModelConnection) {
	ans := &ModelConnection{
		Conn:               rawConn,
		localSubStateCh:    make(chan updateSubTableMsg),
		localSubEventCh:    make(chan updateSubTableMsg),
		remoteSubStateCh:   make(chan updateSubTableMsg),
		remoteSubEventCh:   make(chan updateSubTableMsg),
		querySubState:      make(chan chan []string),
		querySubEvent:      make(chan chan []string),
		removeConnCh:       removeConnCh,
		stateBroadcast:     stateChan,
		eventBroadcast:     eventChan,
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
	addConnChan <- ans
}

func main() {
	fmt.Println("hello proxy")

	go func() {
		connections := make(map[string]*ModelConnection) // 所有连接

		for {
			select {
			case state := <-stateChan:
				for _, conn := range connections {
					conn.stateWriteChan <- state
				}
			case event := <-eventChan:
				for _, conn := range connections {
					conn.eventWriteChan <- event
				}
			case conn := <-addConnChan:
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
			case conn := <-removeConnChan:
				delete(connections, conn.MetaInfo.Name)
				// NOTE: 在此处quitWriter, 不会导致由于连接writer协程体检退出而导致的死锁
				// NOTE: 因为只有调用了quitWriter之后，writer协程才会退出
				conn.quitWriter()
			}
		}
	}()

	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln(err)
	}

	for {
		rawConn, err := l.Accept()
		if err != nil {
			break
		}

		go addModelConnection(rawConn, removeConnChan)
	}

}
