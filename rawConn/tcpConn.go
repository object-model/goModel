package rawConn

import (
	"encoding/binary"
	"net"
	"time"
)

type tcpConn struct {
	*net.TCPConn
}

func (conn *tcpConn) ReadMsg() ([]byte, error) {
	// 读取长度
	var length uint32
	err := binary.Read(conn, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}

	// 读取数据
	data := make([]byte, length)
	if err = binary.Read(conn, binary.LittleEndian, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (conn *tcpConn) WriteMsg(msg []byte) error {
	if len(msg) == 0 {
		return nil
	}

	length := uint32(len(msg))
	err := binary.Write(conn, binary.LittleEndian, &length)
	if err != nil {
		return err
	}

	_, err = conn.Write(msg)
	return err
}

func NewTcpConn(rawConn *net.TCPConn) RawConn {
	_ = rawConn.SetKeepAlive(true)
	_ = rawConn.SetKeepAlivePeriod(time.Second * 5)
	return &tcpConn{
		rawConn,
	}
}
