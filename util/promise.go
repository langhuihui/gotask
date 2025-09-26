package util

import (
	"context"
	"errors"
)

// Promise 实现异步操作的结果
type Promise struct {
	context.Context
	context.CancelCauseFunc
}

// NewPromise 创建一个新的 Promise
func NewPromise(ctx context.Context) *Promise {
	p := &Promise{}
	p.Context, p.CancelCauseFunc = context.WithCancelCause(ctx)
	return p
}

var (
	ErrResolve = errors.New("promise resolved")
)

// Resolve 解决 Promise
func (p *Promise) Resolve() {
	p.Fulfill(nil)
}

// Reject 拒绝 Promise
func (p *Promise) Reject(err error) {
	p.Fulfill(err)
}

// Await 等待 Promise 完成
func (p *Promise) Await() (err error) {
	<-p.Done()
	err = context.Cause(p.Context)
	if errors.Is(err, ErrResolve) {
		err = nil
	}
	return
}

// IsRejected 检查 Promise 是否被拒绝
func (p *Promise) IsRejected() bool {
	return context.Cause(p.Context) != ErrResolve
}

// Fulfill 完成 Promise
func (p *Promise) Fulfill(err error) {
	p.CancelCauseFunc(Conditional(err == nil, ErrResolve, err))
}

// Conditional 条件选择器
func Conditional[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

// Recyclable 可回收接口
type Recyclable interface {
	Recycle()
}
