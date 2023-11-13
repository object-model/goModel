# Proxy

物模型远端代理，通过代理可以使物模型局域云端远程访问能力，实现物模型的广域网护理

# 更新日志

## 20231113

1. 代理添加元信息校验错误事件的推送功能
2. 代理添加物模型名称重复事件的推送功能

## 20231113

**代理添加WebSocket接口的支持**：

1. 按照物模型规范的开发的物模型可以通过WebSocket接口与代理建立连接，进行消息的转发

2. 实现方法简而言之是定义接口`ModelConn`:

   ```go
   type ModelConn interface {
   	Close() error
   	RemoteAddr() net.Addr
   	ReadMsg() ([]byte, error)
   	WriteMsg(msg []byte) error
   }
   ```

   这个接口用于实现如何发送和接收物模型的报文，而模型`model`通过依赖这个接口，实现对不同通信接口的兼容，建立连接时只需构造实现了该接口的对象，并传入`model`中即可

## 20231107

删除Server的退出和完成退出信号，因为一旦Sever的ListenServe失败，整个程序直接退出，没有必要再停止run了

## 20231105

1. 支持链路退出时通知所有等待本链路返回响应报文的链路不用等待，直接返回模型已经退出

2. 模型连接添加写数据的管道

3. 链路的发布订阅关系由Server管理，在收到状态或者事件订阅报文、调用请求报文时等待Server完全添加了链路才向Server推送

   >没有直接在对应报文的回调函数中直接等待，而是另外开启单独协程等待，如果直接等待，在连接一建立的时候发送对应报文时，导致死锁一段时间，进而导致连接关闭

4. 在收到状态报文、事件报文、订阅状态报文、订阅事件报文、调用请求报文时，通过`select`+`default`机制，判断连接已经添加则直接推送，否则再开启单独协程推送，避免大多数情况下，连接已经添加进行多余的开协程操作

5. 添加连接时，如果模型名称重复则直接关闭连接，不添加

## 20231105

**梳理Server的run协程、连接的reader协程、连接的writer协程的依赖关系和退出顺序**

### 依赖关系：

1. reader协程需要将收到的状态、事件、调用、响应转发给run协程
2. reader协程需要更新writer协程中的状态和事件的发布表，以及直接向writer协程发送数据
3. run协程需要向writer协程发送数据
4. writer协程只负责通过链路下发网络数据

### 根据依赖关系，退出顺序必定满足一下规定：

1. writer协程一定最后退出
2. 在reader协程中与run协程通信时，必须判断run协程是否完成退出，若退出了，需要退出reader协程
3. 在reader协程退出的最后一步，通知writer退出，在run没有完成退出时，通过run的removeConnCh退出，以便做一些清理工作，在run已经完全退出的情况下，直接通知writer退出

### 退出顺序总结

**在Server运行中正常退出：**

1. 连接关闭或者接收的报文处理出错
2. reader向Server的删除通道removeConnCh发送链路删除信号
3. Server的run协程接收到删除链路信号，调用链路的quitWriter()，通知writer退出
4. writer退出

**Server主动退出run协程：**

1. run协程在退出前，关闭所有连接，在正式退出后关闭完成退出信号done
2. 正在等待Read的连接由于连接被run关闭了，退出reader协程，或者正在通过管道向run协程发送消息时，收到了run协程的done信号(通过`select`语句)，也会退出reader协程
3. 在退出reader协程之前，直接调用连接的quitWriter()，通知writer退出
4. writer退出
