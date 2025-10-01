package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
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
type MyFirstTask3 struct {
	MyFirstTask
}

// Go 任务运行方法, 使用Go方法实现异步执行,TODO: 取消注释来完成任务运行
// func (t *MyFirstTask3) Go() error {
// 	t.Info("任务正在协程中运行", "name", t.Name())
// 	time.Sleep(2 * time.Second)
// 	return nil
// }

// TestLesson01_3 Go方法的使用
func TestLesson01_3(t *testing.T) {
	t.Log("=== Lesson 1-3: Go方法的使用 ===")
	t.Log("课程目标：学习GoTask框架中Go方法的使用，理解Go方法与Run方法的区别")
	t.Log("关键概念：Go方法会自动进入GOING状态，在独立协程中运行")

	// 创建第三个任务
	myTask3 := &MyFirstTask3{MyFirstTask: MyFirstTask{T: t}}
	root.AddTask(myTask3)

	time.Sleep(1 * time.Second) // 等待任务启动

	if myTask3.GetState() == task.TASK_STATE_GOING {
		t.Log("Lesson 1-3 测试通过：Go方法的使用")
		return
	}
	t.Errorf("课程未通过")
}
