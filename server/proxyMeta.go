package server

// proxyMetaJSON 表示代理的物模型描述元信息
const proxyMetaJSON = `
{
	"type": "meta-info",
	"payload": {
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
                        "description": "校验出错的物模型的IP地址:端口号",
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
                        "description": "名称重复的物模型的IP地址:端口号",
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
                                    "description": "IP地址:端口号",
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
                "description": "获取指定名称的物模型的",
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
                                "description": "IP地址:端口号",
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
                "description": "查询某个物模型是否在线",
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
                "description": "获取某个物模型的状态订阅列表",
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
                "description": "获取某个物模型的事件订阅列表",
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
}
`

// noneMetaJSON 表示空物模型描述元信息
const noneMetaJSON = `
{
	"name": "none",
	"description": "空物模型",
	"state": [],
	"event": [],
	"method": []
}
`
