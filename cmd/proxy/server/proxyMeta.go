package server

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/object-model/goModel/message"
	"strings"
)

// ProxyMetaString 表示代理的物模型描述元信息
const ProxyMetaString = `
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
}`

const NoneMetaString = `
{
	"name": "none",
	"description": "空物模型",
	"state": [],
	"event": [],
	"method": []
}`

// proxyMetaMessage 表示代理的物模型元信息响应报文
var proxyMetaMessage []byte

// noneMetaMessage 表示空物模型描述元信息响应报文
var noneMetaMessage []byte

func init() {
	metaSendData := message.Must(message.EncodeRawMsg("meta-info",
		jsoniter.RawMessage(strings.Join(strings.Fields(ProxyMetaString), ""))))
	proxyMetaMessage = metaSendData
	noneMetaMessage = []byte(strings.Join(strings.Fields(NoneMetaString), ""))
}
