package rawConn

import "net"

type RawConn interface {
	Close() error

	RemoteAddr() net.Addr

	// ReadMsg 从物模型连接中读取完整的一包物模型报文并返回读取的报文和错误信息
	ReadMsg() ([]byte, error)

	// WriteMsg 将物模型报文msg通过连接发送到网络上
	WriteMsg(msg []byte) error
}
