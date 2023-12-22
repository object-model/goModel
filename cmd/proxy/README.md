# Proxy

物模型代理服务proxy可以使物模型具有云端远程访问能力，实现物模型的广域网互联。不同的物模型在与代理服务proxy成功建立连接后，可以通过代理订阅其他物模型的状态和事件报文，远程调用其他物模型的方法。代理服务proxy内部通过处理和转发物模型的报文来达到不同物模型通过代理进行远程既跨局域网的信息交互。同时代理服务本身也是一个标准的物模型，其他物模型也可以直接代理服务的事件，调用代理服务的方法。物模型可以通过TCP或WebSocket与代理服务器建立连接。代理服务proxy提供了丰富的命令行参数来控制服务的启动选项。

# 物模型与代理服务建立连接过程

物模型与代理服务建立连接不只是简单建立TCP或者WebSocket连接就好了，代理服务会在连接建立后对新建立连接的物模型进行一系列检查，只有符合要求才会被正式添加到代理服务中，否则代理服务会拒绝连接，具体步骤如下：

1. 代理服务首先向建立连接的物模型发送模型查询报文，目的是查询对方的元信息；
2. 等待对方返回模型描述信息报文，等待超时时间为5s，若5s内没有收到对方回复的模型描述信息报文，则直接断开连接，不作后续处理；
3. 从对方返回的模型描述信息报文中解析元信息，如果解析失败，则直接断开连接，不作后续处理；
4. 判断元物模型元信息是否符合物模型架构规范，若不满足，则会推送[物模型元信息校验错误事件](#物模型元信息校验错误事件)（也会给这个出错的物模型推送一份），1s后断开连接，不作后续处理；
5. 检查建立连接的物模型的名称是否与代理服务管理的现有物模型冲突，若有冲突，则会推送[物模型名称重复事件](#物模型名称重复事件)（也会给这个冲突的物模型推送一份），1s后断开连接，不作后续处理；
6. 代理服务正式将建立连接的物模型纳入到其管理的物模型列表中，并推送[物模型上线事件](#物模型上线事件)，此时该物模型发送的所有需要处理或者转发的报文（除了模型查询报文和模型描述信息报文以外的所有报文）才能被代理服务器所处理，在此之前发送的所有报文会被缓存至队列中，直到代理服务正式添加了物模型之后，按照入队顺序被依次处理。

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
| `-log`    | 是否将收发的数据保存到日志文件中，若开启，软件启动时会以当前日期时间为文件名，在./logs文件夹下创建日志文件，并将收发数据保存到该文件中 | false        |
| `-meta`   | 是否打印代理服务本身的物模型描述信息，若开启，软件启动时会先打印代理本身的物模型描述信息 | false        |
| `-p`      | 是否将收发的数据打印到控制台中                               | false        |
| `-v`      | 是否打印代理服务的版本号并退出程序                           | false        |
| `-ws`     | 是否开启WebSocket服务，当开启后，物模型可以通过WebSocket与代理服务建立连接 | false        |
| `-wsAddr` | WebSocket监听地址，物模型可以使用WebSocket协议连接到此地址与代理服务建立连接 | 0.0.0.0:9090 |

# 代理服务的物模型

代理服务本身也是一个物模型，本身也提供了一些和代理相关的事件和方法，代理物模型的描述JSON串如下：

```json
{
    "name": "proxy",
    "description": "model proxy service",
    "state": [],
    "event": [
        {
            "name": "online",
            "description": "模型上线事件",
            "args": [
                {
                    "name": "modelName",
                    "description": "上线的物模型名称",
                    "type": "string"
                },
                {
                    "name": "addr",
                    "description": "IP地址:端口号",
                    "type": "string"
                }
            ]
        },

        {
            "name": "offline",
            "description": "模型下线事件",
            "args": [
                {
                    "name": "modelName",
                    "description": "下线的物模型名称",
                    "type": "string"
                },
                {
                    "name": "addr",
                    "description": "IP地址:端口号",
                    "type": "string"
                }
            ]
        },

        {
            "name": "closed",
            "description": "连接关闭事件",
            "args": [
                {
                    "name": "addr",
                    "description": "IP地址:端口号",
                    "type": "string"
                },
                {
                    "name": "reason",
                    "description": "关闭原因",
                    "type": "string"
                }
            ]
        },

        {
            "name": "metaCheckError",
            "description": "物模型元信息校验错误事件",
            "args": [
                {
                    "name": "error",
                    "description": "校验错误提示信息",
                    "type": "string"
                },

                {
                    "name": "modelName",
                    "description": "校验出错的物模型名称",
                    "type": "string"
                },

                {
                    "name": "addr",
                    "description": "校验出错的物模型的地址",
                    "type": "string"
                }
            ]
        },

        {
            "name": "repeatModelNameError",
            "description": "物模型名称重复错误事件",
            "args": [

                {
                    "name": "modelName",
                    "description": "名称重复的物模型名称",
                    "type": "string"
                },

                {
                    "name": "addr",
                    "description": "名称重复的物模型的地址",
                    "type": "string"
                }
            ]
        }
    ],
    "method": [
        {
            "name": "GetAllModel",
            "description": "获取本代理下当前在线的所有物模型信息",
            "args": [],
            "response": [
                {
                    "name": "modelList",
                    "description": "在线的物模型信息列表",
                    "type": "slice",
                    "element": {
                        "type": "struct",
                        "fields": [
                            {
                                "name": "modelName",
                                "description": "物模型名称",
                                "type": "string"
                            },
                            {
                                "name": "addr",
                                "description": "地址",
                                "type": "string"
                            },
                            {
                                "name": "subStates",
                                "description": "状态订阅列表",
                                "type": "slice",
                                "element": {
                                    "type": "string"
                                }
                            },
                            {
                                "name": "subEvents",
                                "description": "事件订阅列表",
                                "type": "slice",
                                "element": {
                                    "type": "string"
                                }
                            },
                            {
                                "name": "metaInfo",
                                "description": "模型元信息",
                                "type": "meta"
                            }
                        ]
                    }
                }
            ]
        },

        {
            "name": "GetModel",
            "description": "获取指定名称的物模型的信息",
            "args": [
                {
                    "name": "modelName",
                    "description": "物模型名称",
                    "type": "string"
                }
            ],

            "response": [
                {
                    "name": "modelInfo",
                    "description": "物模型信息",
                    "type": "struct",
                    "fields": [
                        {
                            "name": "modelName",
                            "description": "物模型名称",
                            "type": "string"
                        },
                        {
                            "name": "addr",
                            "description": "地址",
                            "type": "string"
                        },
                        {
                            "name": "subStates",
                            "description": "状态订阅列表",
                            "type": "slice",
                            "element": {
                                "type": "string"
                            }
                        },
                        {
                            "name": "subEvents",
                            "description": "事件订阅列表",
                            "type": "slice",
                            "element": {
                                "type": "string"
                            }
                        },
                        {
                            "name": "metaInfo",
                            "description": "模型元信息",
                            "type": "meta"
                        }
                    ]
                },

                {
                    "name": "got",
                    "description": "是否获取成功，不在线返回false",
                    "type": "bool"
                }
            ]

        },

        {
            "name": "ModelIsOnline",
            "description": "查询指定名称的物模型是否在线",
            "args": [
                {
                    "name": "modelName",
                    "description": "物模型名称",
                    "type": "string"
                }
            ],
            "response": [
                {
                    "name": "isOnline",
                    "description": "是否在线",
                    "type": "bool"
                }
            ]
        },

        {
            "name": "GetSubState",
            "description": "获取指定名称的物模型的状态订阅列表",
            "args": [
                {
                    "name": "modelName",
                    "description": "物模型名称",
                    "type": "string"
                }
            ],
            "response": [
                {
                    "name": "subList",
                    "description": "状态订阅列表",
                    "type": "slice",
                    "element": {
                        "type": "string"
                    }
                },
                {
                    "name": "got",
                    "description": "是否获取成功，不在线返回false",
                    "type": "bool"
                }
            ]
        },

        
        {
            "name": "GetSubEvent",
            "description": "获取指定名称的物模型的事件订阅列表",
            "args": [
                {
                    "name": "modelName",
                    "description": "物模型名称",
                    "type": "string"
                }
            ],
            "response": [
                {
                    "name": "subList",
                    "description": "事件订阅列表",
                    "type": "slice",
                    "element": {
                        "type": "string"
                    }
                },
                {
                    "name": "got",
                    "description": "是否获取成功，不在线返回false",
                    "type": "bool"
                }
            ]
        }
    ]
}
```

## 事件

### 物模型上线事件

- **事件名：**`proxy/online`
- **作用：**通知感兴趣的物模型，代理服务添加了新的物模型
- **触发时机：**当物模型与代理服务建立连接，并且其元信息检查无误，同时名称又不与代理服务管理的其他物模型冲突时，会触发此事件
- **参数：**上线的物模型名称、地址信息

### 物模型下线事件

- **事件名：**`proxy/offline`
- **作用：**通知感兴趣的物模型，代理服务从其管理的物模型中删除了某个物模型
- **触发时机：**当被代理服务管理的物模型由于某种原因需要断开连接时，代理会删除此物模型，同时触发该事件
- **参数：**删除的物模型名称、地址信息

### 连接关闭事件

- **事件名：**`proxy/closed`
- **作用：**通知感兴趣的物模型，物模型连接已经关闭
- **触发时机：**当连接与代理服务由于某种原因断开连接时，会触发该事件
- **参数：**连接的地址信息和断开原因

### 物模型元信息校验错误事件

- **事件名：**`proxy/metaCheckError`
- **作用：**通知感兴趣的物模型，某个刚建立连接的物模型的元信息校验不通过
- **触发时机：**当代理服务发现刚建立连接的物模型的元信息不符合物模型架构规范时，会触发该事件
- **参数：**校验不通过的物模型名称、地址信息和校验错误提示信息

### 物模型名称重复事件

- **事件名：**`proxy/repeatModelNameError`
- **作用：**通知感兴趣的物模型，某个刚建立连接的物模型的名称和其他的物模型有冲突
- **触发时机：**当代理服务发现刚建立连接的物模型的名称与其管理的其他物模型的名称重复时，会触发该事件
- **参数：**名称重复的物模型名称、地址信息

## 方法

### 获取本代理下当前在线的所有物模型信息

- **方法名：**`proxy/GetAllModel`
- **作用：**获取本代理下当前在线的所有物模型信息
- **参数：**无
- **返回：**包含物模型信息的不定长数组，数组中每一项为包含物模型名称、地址、状态订阅列表、事件订阅列表、元信息的对象

### 获取指定名称的物模型信息

- **方法名：**`proxy/GetModel`

- **作用：**获取指定名称的物模型的信息

- **参数：**待查询的物模型名称

- **返回：**包含两个返回值，第一个是查询的物模型的信息对象，第二个为是否获取成功的bool值，如果查询的物模型不在线则为false

- **备注：**若查询的物模型不存在，则该返回的第一个返回值为以下JSON表示的空物模型：

  ```json
  {
      "modelName": "none",
      "addr": "",
      "subStates": [],
      "subEvents": [],
      "metaInfo": {
          "name": "none",
          "description": "空物模型",
          "state": [],
          "event": [],
          "method": []
  	}
  }
  ```

### 查询指定名称的物模型是否在线

- **方法名：**`proxy/ModelIsOnline`
- **作用：**查询指定名称的物模型是否在线
- **参数：**待查询的物模型名称
- **返回：**表示物模型是否在线的bool值

### 获取指定名称的物模型的状态订阅列表

- **方法名：**`proxy/GetSubState`
- **作用：**获取指定名称的物模型的状态订阅列表
- **参数：**待查询的物模型名称
- **返回**：包含两个返回值，第一个为字符串类型的不定长列表，表示所查询的物模型的状态订阅列表，第二个参数为是否获取成功的bool值，若物模型不在线则返回false

### 获取指定名称的物模型的事件订阅列表

- **方法名：**`proxy/GetSubEvent`
- **作用：**获取指定名称的物模型的事件订阅列表
- **参数：**待查询的物模型名称
- **返回**：包含两个返回值，第一个为字符串类型的不定长列表，表示所查询的物模型的事件订阅列表，第二个参数为是否获取成功的bool值，若物模型不在线则返回false
