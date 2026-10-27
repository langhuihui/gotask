//go:build lesson

package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask/v2"
)

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, task.ManagerItem[uint32]]

// 创建根任务管理器
var root TaskManager

func init() {
	root.Init()
}

// MyFirstTask 第一个任务示例
type MyFirstTask struct {
	task.Task
	*testing.T
}

// Start 任务启动方法
func (t *MyFirstTask) Start() error {
	t.Log("任务开始执行", t.Name())
	return nil
}

// Dispose 任务清理方法
func (t *MyFirstTask) Dispose() {
	t.Log("任务清理", t.Name())
}

// TestLesson01 测试基础Task使用
func TestLesson01_1(t *testing.T) {
	t.Log("=== Lesson 1-1: 基础Task使用 ===")
	t.Skip("练习：完成下方 TODO（然后删除本行）即可验证本课")
	t.Log("课程目标：学习GoTask框架中最基本的Task使用方法")
	t.Log("关键概念：任务需要使用AddTask才能运行（父任务驱动子任务运行）")

	// 创建第一个任务
	myTask := &MyFirstTask{T: t}

	//TODO: 取消下面的注释来完成任务添加
	// root.AddTask(myTask)

	time.Sleep(1 * time.Second) // 等待任务启动

	if myTask.GetState() == task.TASK_STATE_STARTED {
		t.Log("Lesson 1-1 测试通过：基础Task使用")
		return
	}
	t.Errorf("课程未通过")
}

// MyFirstTask2 第二个任务示例,将继承MyFirstTask的 Start 和 Dispose 方法
type MyFirstTask2 struct {
	MyFirstTask
}

// Run 任务运行方法，TODO: 取消注释来完成任务运行
// func (t *MyFirstTask2) Run() error {
// 	t.Info("任务正在运行", "name", t.Name())
// 	time.Sleep(2 * time.Second)
// 	return nil
// }

// TestLesson01_2 Run方法的使用
func TestLesson01_2(t *testing.T) {
	t.Log("=== Lesson 1-2: Run方法的使用 ===")
	t.Skip("练习：完成下方 TODO（然后删除本行）即可验证本课")
	t.Log("课程目标：学习GoTask框架中继承Task的使用方法和Run 方法的使用")
	t.Log("关键概念：Run方法会自动进入RUNNING状态")

	// 创建第二个任务
	myTask2 := &MyFirstTask2{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask2)

	time.Sleep(1 * time.Second) // 等待任务启动

	if myTask2.GetState() == task.TASK_STATE_RUNNING {
		t.Log("Lesson 1-2 测试通过：Run方法的使用")
		return
	}
	t.Errorf("课程未通过")
}

// MyFirstTask3 第三个任务示例
//
// v2 起 Go() 不再是任务的执行入口：TaskGo 已删除，Run 是唯一入口（见
// docs/重构-执行模型v2-大版本v2设计.md §4.1）。把它写成 Go() 不会编译报错，
// 但框架会以 ErrLegacyTaskGo 明确失败——而不是让任务「启动成功却永远不执行」。
type MyFirstTask3 struct {
	MyFirstTask
}

// Go 故意沿用 v1 的方法名，用来观察 v2 的迁移守卫。
func (t *MyFirstTask3) Go() error {
	t.Info("这段代码在 v2 中不会被执行", "name", t.Name())
	return nil
}

// TestLesson01_3 v2 迁移守卫：Go() 不再是执行入口
func TestLesson01_3(t *testing.T) {
	t.Log("=== Lesson 1-3: Go() 不再是任务执行入口（v2） ===")
	t.Skip("练习：删除上方 Skip 行即可验证本课")
	t.Log("课程目标：理解 v2 为什么删除 TaskGo，以及迁移守卫如何保护你")
	t.Log("关键概念：只实现 Go() 的任务会以 ErrLegacyTaskGo 停止，而不是静默空转")

	// 创建第三个任务
	myTask3 := &MyFirstTask3{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask3)

	time.Sleep(1 * time.Second) // 等待迁移守卫触发

	if myTask3.StopReasonIs(task.ErrLegacyTaskGo) {
		t.Log("Lesson 1-3 测试通过：迁移守卫捕获了遗留的 Go() 任务")
		return
	}
	t.Errorf("课程未通过：期望 ErrLegacyTaskGo，实际 %v", myTask3.StopReason())
}
