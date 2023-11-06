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
            }
		],
		"method": [
            {
                "name": "GetAllModel",
                "description": "获取本代理下当前在线的物模型名称和地址列表",
                "args": [],
                "response": [
                    {
                        "name": "modelList",
                        "description": "在线的物模型名称+地址列表",
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
                                    "name": "meta",
                                    "description": "模型元信息",
                                    "type": "meta"
                                }
                            ]
                        }
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
            },
            
            {
                "name": "GetAllModelMeta",
                "description": "获取本代理下当前在线的物模型元信息列表",
                "args": [
                ],
                "response": [
                    {
                        "name": "metaList",
                        "description": "在线的物模型元信息列表",
                        "type": "slice",
                        "element": {
                            "type": "meta"
                        }
                    }
                ]
            },

            {
                "name": "GetModelMeta",
                "description": "获取指定物模型的元信息",
                "args": [
                    {
                        "name": "modelName",
                        "description": "物模型名称",
                        "type": "string"
                    }
                ],
                "response": [
                    {
                        "name": "metaInfo",
                        "description": "获取的物模型的元信息",
                        "type": "meta"
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
