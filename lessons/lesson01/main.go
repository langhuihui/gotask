package main

import (
	"fmt"

	task "github.com/langhuihui/gotask"
)

// MyFirstTask 第一个任务示例
// 这个任务演示了最基本的Task使用方法
type MyFirstTask struct {
	task.Task
	Name string
}

// Start 任务启动方法
// 当任务开始执行时会调用此方法
func (t *MyFirstTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// t.Info("任务开始执行", "name", t.Name)
	// fmt.Printf("Hello, %s! 任务已启动\n", t.Name)

	// 验证：检查是否完成了TODO
	if t.Name == "" {
		fmt.Println("❌ 错误：请先设置任务名称")
		return fmt.Errorf("任务名称未设置")
	}

	// 验证：检查是否取消了注释
	if t.Logger == nil {
		fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
		return fmt.Errorf("Start方法未完成")
	}

	return nil
}

// Run 任务运行方法
// 任务的主要业务逻辑在这里执行
func (t *MyFirstTask) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// t.Info("任务正在运行", "name", t.Name)
	// fmt.Printf("任务 %s 正在执行中...\n", t.Name)
	// time.Sleep(2 * time.Second)
	// fmt.Printf("任务 %s 执行完成\n", t.Name)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

// Dispose 任务清理方法
// 当任务结束时调用此方法进行资源清理
func (t *MyFirstTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// t.Info("任务清理", "name", t.Name)
	// fmt.Printf("任务 %s 已清理完成\n", t.Name)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *MyFirstTask]

func main() {
	fmt.Println("=== GoTask Lesson 1: 基础Task使用 ===")
	fmt.Println("本课程将教你如何使用最基本的Task功能")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建第一个任务
	myTask := &MyFirstTask{Name: "我的第一个任务"}

	// 将任务添加到根管理器中
	root.AddTask(myTask)

	// 等待任务完成
	err := myTask.WaitStopped()

	if err != nil {
		fmt.Printf("❌ 任务执行失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
	} else {
		fmt.Println("=== 课程完成 ===")
		fmt.Println("如果看到任务执行日志，说明你已经成功完成了Lesson 1!")
	}

	// 优雅关闭
	root.Shutdown()
}
