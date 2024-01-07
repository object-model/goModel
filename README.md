# goModel

golang版本的物模型，包含物模型框架、远端代理服务和代码生成工具.

# 更新日志

## 20240107

1. 解决代理在正式添加连接前缓存报文处理逻辑的bug

   出现原因是客户端与代理连接后立即同步调用方法，此时代理还没有正式添加连接：

   ```go
   proxy := m.Dial("tcp@localhost:8080")
   rawResp, err := proxy.Call("calc/sqrt", message.Args{
     "x": 100,
   })
   ```

   由于客户端一直在等待响应而一只没有向代理发送任何报文，因此在代理端，连接的`reader`方法一直处于挂起状态，因此，即使代理已经正式添加了连接，但由于`reader`处于阻塞态，之前缓存的报文也一直不能处理，从而导致客户端一直无法解除等待。

   解决方法：代理正式添加连接之后，主动发送一包元信息查询报文，确保`reader`解除阻塞，从而将缓存的报文处理掉：

   ```go
   	// 添加链路, 并通知已添加
   	connections[m.MetaInfo.Name] = conn
   	m.setAdded()
   
   	// NOTE: 目的是立即唤醒reader, 保证缓存的报文能及时处理
   	m.writeChan <- message.EncodeQueryMetaMsg()
   
   ```

2. 优化代理的报文处理流程，使用map代替switch-case

## 20231228

1. 完成包model的所有接口注释说明

2. 补充包meta的接口注释

## 20231225

1. 删除停止重连回调，没有必要

2. 修改设置连接时，判断连接是否有效的逻辑反转错误

3. 添加是否有效接口

4. 自动重连回调添加取消自动重建的参数

   ```go
   type OnReConnect func(cancel func(), num uint, ok bool)
   ```

   调用`cancel()`可以取消自动重连.

## 20231224

1. 完成自动重连相关功能
2. 物模型添加统一的建立连接接口Dial
3. 代理创建物模型时的元信息赋值为空元信息，以防止访问为nil的元信息

   原先的做法是元信息默认为空，这样会导致在代理还未获取到其元信息之前，访问元信息字段会导致空指针访问!

   ```go
   func (s *Server) onRemoveConn(connections map[string]connection, m *model,
   	respWaiters map[string]string) {
   	
       // NOTE: 此时还未获取到m的元信息, 物模型连接连接就早早地要删除和关闭了
       // NOTE: 此时m.MetaInfo为nil, 因此会导致空指针访问
       if conn, seen := connections[m.MetaInfo.Name]; seen && m.isAdded() {
           ...
       }
       ...
   }
   ```

## 20231222

1. 简化代理在正式添加物模型之前的转发报文处理逻辑

   直接使用缓冲区缓存需要代理转发的报文，不开启单独的协程

## 20231221

1. 完成Call、CallFor、GetPeerMeta在连接发送报文失败场景下行为的单元测试
2. 完成真实环境下异步+回调函数的调用场景的测试
3. 完成真实环境下异步+回调+超时的调用场景的测试
4. 优化测试响应回调再规定时间内都执行的测试方法，保证在-race选项开启后也能执行通过
5. 完成调用请求回调函数、状态回调函数、事件回调函数的逻辑测试
6. 在调用关闭回调之后再关闭原始连接，确保主动关闭连接时，关闭原因始终为active close
7. 完成真实环境下远程调用后主动关闭连接场景下的测试
8. 简化CallCloseSuite退出逻辑
9. 规范断言的使用，避免出现datarace
10. 减少执行和等待时间，减少测试执行时间

## 20231220

1. 测试真实环境下同步调用

2. 测试真实环境下同步+超时调用

3. 物模型的ListenServeWebSocket启动http服务使用单独的路由配置对象，不使用http包默认的路由配置对象

   ```go
   func (m *Model) ListenServeWebSocket(addr string) error {
   	mux := http.NewServeMux()
   	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
   		conn, err := upgrader.Upgrade(writer, request, nil)
   		if err != nil {
   			return
   		}
   
   		m.dealConn(newConn(m, rawConn.NewWebSocketConn(conn)))
   	})
   	return http.ListenAndServe(addr, mux)
   }
   ```

   **目的：**原先的做法在一个程序内开两个物模型并都运行WebSocket服务时会奔溃，**因为都向http包中的默认路由`DefaultServeMux`注册了相同的路径！**

## 20231219

1. 将uuid生成提取成Connection的一个成员，便于打桩测试

2. 完成连接异步调用接口的单元测试

## 20231218

1. 完成发送状态和事件订阅接口的单元测试
1. 若调用请求回调返回nil, 则将响应改为空对象, 补充相关测试用例

## 20231217

1. 状态、事件、调用请求和调用响应报文在JSON反序列化后，进行相关字段的空值判断，防止字段不存在或为零值的情况
2. 完成无效的调用请求报文的处理逻辑的测试
3. 补充无效的状态、事件报文的测试用例，以测试字段值不存在或者为零值的情况
4. 完成响应报文的处理逻辑的测试
5. 完成元信息报文的处理逻辑测试
6. 完善收到无效JSON数据时的测试用例，考虑此种情况下的响应等待和对端元信息等待的处理逻辑

## 20231216

1. 完成物模型Model推送状态和事件的单元测试
2. 根据单元测试修复推送事件的bug
3. 完成状态和事件订阅报文处理逻辑的单元测试
4. 状态、事件、调用请求、连接关闭回调函数都改成接口的方式，以便测试和扩展
5. 完成状态、事件报文处理逻辑的单元测试
6. 完成模型配置相关接口单元测试
7. 完成有效调用请求报文的测试

## 20231213

1. 连接异步调用添加回调形式的接口
2. 连接异步调用添加回调+超时形式的接口
3. 连接异步调用接口名改成`Invoke`，同步调用改成`Call`
4. 连接关闭时解除所有调用等待和获取对端元信息等待
5. 修改报文处理逻辑错误：元信息查询报文和元信息报文处理逻辑搞混了

## 20231207

1. 完成根据元信息校验原始事件参数的测试用例

2. 完成根据元信息校验原始调用请求参数的测试用例

3. 完成根据元信息校验原始调用响应返回值的测试用例

4. 判断原始数据是否为有效的JSON数据的方法改成直接调用`Unmarshal`：

   ```go
   	var value interface{}
   	if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, &value); err != nil {
   		return fmt.Errorf("invalid JSON data")
   	}
   ```

   之前的方法为：

   ```go
   	it := jsoniter.ParseBytes(jsoniter.ConfigCompatibleWithStandardLibrary, data)
   	root := it.ReadAny()
   	if (it.Error != nil && it.Error != io.EOF) || it.WhatIsNext() != jsoniter.InvalidValue {
   		return fmt.Errorf("invalid JSON data")
   	}
   ```

   之前的方法在输入数据为某些极端数据时不会返回错误，例如`123_45`、`{}123`、`123true"name"`，改进后的做法能杜绝这种情况的发生

5. 添加新的测试用例，充分验证无效JSON数据的情况

6. 将所有的JSON序列化与反序列化都改成与标准库兼容的方式

7. 将message中编码物模型报文的方式改成一次编码

   ```go
   	msg := Message{
   		Type:    typeStr,
   		Payload: items,
   	}
   
   	json := jsoniter.ConfigCompatibleWithStandardLibrary
   	ans, _ := json.Marshal(msg)
   ```

8. 完成包message的单元测试

## 20231206

1. 完成根据元信息校验原始状态报文的功能

2. 完成根据元信息校验原始状态数据的测试用例

3. 在校验原始数据的第一步解析JSON串时，排除返回`io.EOF`的情况：

   ```go
   	it := jsoniter.ParseBytes(jsoniter.ConfigCompatibleWithStandardLibrary, data)
   	if (it.Error != nil && it.Error != io.EOF) || it.WhatIsNext() != jsoniter.InvalidValue {
   		return fmt.Errorf("invalid JSON data")
   	}
   ```

   原因是数值类型的JSON数据例如`123`，解析后返回的是`io.EOF`。

4. 使用和标准库完全兼容的解码方式

5. 完成根据元信息校验原始事件参数的功能

6. 完成根据元信息校验原始调用请求参数的功能

7. 完成根据元信息校验原始调用响应返回值的功能

## 20231205

1. 优化程序模块划分，将所有物模型报文的编码都整合到message中，将不属于message的部分移动到别处
1. 元信息添加序列化成JSON串的接口，且有缓存功能，最多只序列化一次
1. 根据元信息校验事件参数、方法参数和方法返回值采用message中的类型
1. 校验浮点数据支撑int、uint兼容的类型

## 20231203

1. 物模型元信息解析失败返回一个包含uuid模板的空的物模型

2. 完成根据元信息校验状态的功能

3. 完成状态校验的部分测试用例

4. 完成根据事件元信息校验事件的功能

5. 完成事件校验的测试用例

6. 校验数据时，添加是否校验范围的选项。目的是在校验数组或者切片的元素类型时不会因为范围出错而导致类型错误，数组或者切片的元素类型的校验是通过反射创建的变量来校验的，在创建时没有代入范围信息

   ```go
   	zeroElem := reflect.New(reflect.TypeOf(data).Elem()).Elem().Interface()
   	if err := _verifyData_(*meta.Element, zeroElem, false); err != nil {
   		return fmt.Errorf("element: %s", err)
   	}
   ```

7. 数组或切片类型的参数在逐个校验每个元素之前，先通过反射机制校验其元素类型是否符合元信息！如果不这么做，在类型为切片，数据的切片长度为0的情况下，错误的元素类型也能校验通过；在类型为数组时，报错信息不合适

8. 切片类型的校验，添加不能为`nil`的先决条件，保证编码后始终为数组`[]`

## 20231202

1. 元信息的解析支持去除多余的空格以增强适用性
2. 测试用例增加包含多余空格的情况
3. 完成元信息加载解析支持模板参数的功能
4. 完成和元信息模板参数相关的单元测试
5. 所有去除空格都改成TrimSpace
6. 元信息中为状态、事件和方法新建名称到下标的映射，便于后期按名称查询相关元信息

## 20231201

1. 将物模型元信息的解析改成和C++版本的物模型框架一致的方式
2. 完成元信息解析的单元测试
3. 代理检查物模型改用新的接口

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
