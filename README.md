# Proxy

物模型代理服务proxy可以使物模型具有云端远程访问能力，实现物模型的广域网互联。不同的物模型在与代理服务proxy成功建立连接后，可以通过代理订阅其他物模型的状态和事件报文，远程调用其他物模型的方法。代理服务proxy内部通过处理和转发物模型的报文来达到不同物模型通过代理进行远程既跨局域网的信息交互。同时代理服务本身也是一个标准的物模型，其他物模型也可以直接代理服务的事件，调用代理服务的方法。物模型可以通过TCP或WebSocket与代理服务器建立连接。代理服务proxy提供了丰富的命令行参数来控制服务的启动选项。

# 物模型与代理服务建立连接过程

物模型与代理服务建立连接不只是简单建立TCP或者WebSocket连接就好了，代理服务会在连接建立后对新建立连接的物模型进行一系列检查，只有符合要求才会被正式添加到代理服务中，否则代理服务会拒绝连接，具体步骤如下：

1. 代理服务首先向建立连接的物模型发送模型查询报文，目的是查询对方的元信息；
2. 等待对方返回模型描述信息报文，等待超时时间为5s，若5s内没有收到对方回复的模型描述信息报文，则直接断开连接，不作后续处理；
3. 从对方返回的模型描述信息报文中解析元信息，如果解析失败，则直接断开连接，不作后续处理；
4. 判断元物模型元信息是否符合物模型架构规范，若不满足，则会推送物模型元信息校验错误事件（也会给这个出错的物模型推送一份），1s后断开连接，不作后续处理；
5. 检查建立连接的物模型的名称是否与代理服务管理的现有物模型冲突，若有冲突，则会推送物模型名称冲突事件（也会给这个冲突的物模型推送一份），1s后断开连接，不作后续处理；
6. 代理服务正式将建立连接的物模型纳入到其管理的物模型列表中，此时该物模型发送的所有需要处理或者转发的报文（除了模型查询报文和模型描述信息报文以外的所有报文）才能被代理服务器所处理，在此之前发送的所有报文会被缓存至队列中，直到代理服务正式添加了物模型之后，按照入队顺序被依次处理。

# 命令行参数

代理服务提供了丰富的命令行参数，用于控制代理服务的运行配置。用户可以通过运行`./proxy -help`查看代理服务的使用说明：

```c++
Usage of ./proxy:
  -addr string
        proxy tcp address (default "0.0.0.0:8080")
  -log
        whether to save send and received message to file
  -meta
        show proxy meta info
  -p    whether to print send and received message on console
  -v    show version of proxy and quit
  -ws
        whether to run websocket service
  -wsAddr string
        proxy websocket address (default "0.0.0.0:9090")

Proxy is object model proxy server which can transmit model message and also provides methods and events itself. Model can connect to proxy using tcp or websocket interface.
```

代理服务所包含的命令行参数如下：

| 参数      | 含义                                                         | 默认值       |
| --------- | ------------------------------------------------------------ | ------------ |
| `-addr`   | 代理服务的TCP监听地址，物模型可以使用TCP协议连接到此地址与代理服务建立连接 | 0.0.0.0:8080 |
| `-log`    | 是否将收发的数据保存到日志文件中                             | false        |
| `-meta`   | 是否打印代理服务本身的物模型描述信息                         | false        |
| `-p`      | 是否将收发的数据打印到控制台中                               | false        |
| `-v`      | 是否打印代理服务的版本号并退出程序                           | false        |
| `-ws`     | 是否开启WebSocket服务，当开启后，物模型可以通过WebSocket与代理服务建立连接 | false        |
| `-wsAddr` | WebSocket监听地址，物模型可以使用WebSocket协议连接到此地址与代理服务建立连接 | 0.0.0.0:9090 |

# 代理服务的物模型



# 更新日志

## 20231126

1. 确保在代理添加模型后收到的报文能在缓存的报文处理完毕后处理，或者在缓存的报文处理出错后直接退出
2. reader函数在退出时，保证bufferMsgHandler无论在是否添加的情况下都可以退出

## 20231125

1. 将代理服务正式添加物模型之前，所有需要处理或转发的报文都缓存到管道中，并在专门的协程中处理。
2. 在代理服务正式添加物模型之后，所有要处理或转发的报文直接处理，不过要等到所有缓存的报文处理完，或者发生错误而退出

## 20231122

1. 添加打印代理元信息的命令选项
2. 优化代理物模型的字符串表示

## 20231120

1. 修复保存元信息时层级错误

   原来错误的做法是，保存元信息报文时保存的是全报文的数据:

   ```go
   func (m *model) onMetaInfo(payload []byte, fullData []byte) error {
   	var metaInfo meta.Meta
   	if err := jsoniter.Unmarshal(payload, &metaInfo); err != nil {
   		return err
   	}
   
   	m.onGetMetaOnce.Do(func() {
   		m.MetaInfo = message.MetaMessage{
   			Meta:     metaInfo,
   			FullData: fullData,
   		}
   		close(m.metaGotChan)
   	})
   
   	return nil
   }
   ```

   这种做法多过保存了一层，即`{"type": "meta-info", "payload": ...}`，不符合格式要求，应该只保存`"payload"`字段中的JSON串。所有该成这样：

   ```go
   func (m *model) onMetaInfo(payload []byte) error {
   	var metaInfo meta.Meta
   	// 去除多余的空格保持兼容性
   	payload = []byte(strings.Join(strings.Fields(string(payload)), ""))
   
   	// 解析
   	if err := jsoniter.Unmarshal(payload, &metaInfo); err != nil {
   		return err
   	}
   
   	m.onGetMetaOnce.Do(func() {
   		m.MetaInfo = message.MetaMessage{
   			Meta:    metaInfo,
   			RawData: payload,
   		}
   		close(m.metaGotChan)
   	})
   
   	return nil
   }
   ```


## 20231119

1. 添加收发数据日志记录功能

## 20231117

1. 推送物模型名称重复事件最后补充**关闭连接动作**，原因是忘记了
2. 修复新加入的物模型与现有物模型名称重复时程序异常崩溃的bug
   - **出现原因：**当名称重复时，Server会调用新加入物模型的`Close()`方法，使其`reader()`函数退出，从而向Server的删除连接管道发出连接删除信号，而Server在收到此信号后会并没有判断发出信号的物模型是没有添加的（由于名称重复），而是直接根据其名字作为key，删除了原先已经添加的物模型，导致后续被误删的物模型发送调用报文时出现空指针的引用！
   - **解决方法：**Server在收到删除连接信号时，判断发出信号的物模型是否已经添加，只有添加了才能将其从Server内部维护的连接列表中删除，否则只执行关闭物模型的writer的动作！

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
