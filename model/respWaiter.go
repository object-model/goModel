package model

import (
	"errors"
	"goModel/message"
	"sync"
	"time"
)

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

func (w *RespWaiter) Wait() (message.RawResp, error) {
	<-w.got
	return w.resp, w.err
}

func (w *RespWaiter) WaitFor(timeout time.Duration) (message.RawResp, error) {
	select {
	case <-time.After(timeout):
		return message.RawResp{}, errors.New("timeout")
	case <-w.got:
		return w.resp, w.err
	}
}
