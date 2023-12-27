package model

import (
	"errors"
	"github.com/object-model/goModel/message"
	"sync"
	"time"
)

// RespWaiter 为调用响应等待器, 用于等待调用请求报文的响应报文.
type RespWaiter struct {
	gotOnce sync.Once       // 保证 got 只关闭一次
	got     chan struct{}   // 收到响应信号
	resp    message.RawResp // 响应原始报文
	err     error           // 响应错误信息
}

func (w *RespWaiter) wake(resp message.RawResp, err error) {
	w.gotOnce.Do(func() {
		w.resp = resp
		w.err = err
		close(w.got)
	})
}

// Wait 阻塞式地等待调用响应报文,直到收到调用响应报文或者连接关闭,返回响应报文的返回值和错误信息.
func (w *RespWaiter) Wait() (message.RawResp, error) {
	<-w.got
	return w.resp, w.err
}

// WaitFor 阻塞式地等待调用响应报文,直到收到调用响应报文、等待时间超过timeout或者连接关闭,
// 返回响应报文的返回值和错误信息.
func (w *RespWaiter) WaitFor(timeout time.Duration) (message.RawResp, error) {
	select {
	case <-time.After(timeout):
		return message.RawResp{}, errors.New("timeout")
	case <-w.got:
		return w.resp, w.err
	}
}
