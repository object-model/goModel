# Proxy

物模型远端代理，通过代理可以使物模型局域云端远程访问能力，实现物模型的广域网护理

# 更新日志

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
