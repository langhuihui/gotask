//go:build v2

package task

// v2 公开 API 面的编译期锁定。
//
// 这是「契约先行」的编译期形式：本文件用的是 v2 **目标形态**的符号，因此在实施
// 前（TaskRun 尚未落地）用 `go build -tags v2 ./...` 必然编译失败——这就是预期
// 的红灯。S3/S4 完成、符号到位后转绿，届时去掉 build tag 让它成为常驻的 API 面
// 看门狗：任何人把已删除的 API 加回来、或往 TaskRun 里夹带新方法，都会在这里
// 编译失败。
//
// 只做「必须存在 / 必须不多不少」的正向与反向断言。符号**必须消失**那一类
// （FromPointer / GetTaskPointer / Singleton / AnyTaskFunc / GetKey /
// TASK_STATE_GOING）无法用编译期断言表达，由验收门 A7 的 `go doc` 核验兜底。

import (
	"context"
	"errors"
)

// ---------- V-A：TaskRun 是唯一的执行入口 ----------

// TaskRun 必须就是 interface{ Run() error }：任何满足后者的值都能直接当
// TaskRun 用。若有人往 TaskRun 里加第二个方法，这一行就编译不过——加方法等于
// 把「只实现 Run 的既有任务」静默踢出调度，正是 v2 要消灭的那类隐式规则。
func v2AnyRunIsTaskRun(r interface{ Run() error }) TaskRun { return r }

type v2RunOnly struct{ Task }

func (*v2RunOnly) Run() error { return nil }

var (
	_ TaskRun = (*v2RunOnly)(nil)   // 手写任务
	_ TaskRun = (*GenericTask)(nil) // 泛型桥（AsTaskBlock 的产物）
	_         = v2AnyRunIsTaskRun
)

// ---------- V-A：迁移守卫的错误值可被 errors.Is 匹配 ----------

var (
	_ = errors.Is(ErrLegacyTaskGo, ErrLegacyTaskGo)
	_ = errors.Is(ErrNotTickTask, ErrNotTickTask)
)

// ---------- V-B：ID 是唯一身份，WithCallDepth 是唯一的调用栈深度入口 ----------

func v2TaskIdentity(t *Task) uint32 { return t.GetTaskID() }

var (
	_ = v2TaskIdentity
	_ = WithCallDepth(2)
	_ = WithContext(context.Background())
)
