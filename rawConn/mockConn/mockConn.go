package mockConn

import (
	"github.com/stretchr/testify/mock"
	"net"
)

// 模拟连接
type MockConn struct {
	mock.Mock
}

func (m *MockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) ReadMsg() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockConn) WriteMsg(msg []byte) error {
	args := m.Called(msg)
	return args.Error(0)
}
