package main

import (
	"fmt"

	task "github.com/langhuihui/gotask"
)

// MessageTask 消息处理任务
type MessageTask struct {
	task.ChannelTask
	MessageChan chan string
	TaskName    string
}

func (t *MessageTask) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// t.Info("消息任务启动", "taskName", t.TaskName)
	// fmt.Printf("消息任务 %s 已启动\n", t.TaskName)
	//
	// // 创建消息通道
	// t.MessageChan = make(chan string, 10)
	// t.SignalChan = t.MessageChan

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (t *MessageTask) Go() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// t.Info("消息任务运行中", "taskName", t.TaskName)
	// fmt.Printf("消息任务 %s 正在运行...\n", t.TaskName)
	//
	// for {
	// 	select {
	// 	case msg := <-t.MessageChan:
	// 		fmt.Printf("消息任务 %s 收到消息: %s\n", t.TaskName, msg)
	// 	case <-t.Done():
	// 		fmt.Printf("消息任务 %s 收到停止信号\n", t.TaskName)
	// 		return nil
	// 	}
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Go方法中的TODO注释")
	return fmt.Errorf("Go方法未完成")
}

func (t *MessageTask) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// t.Info("消息任务清理", "taskName", t.TaskName)
	// fmt.Printf("消息任务 %s 已清理\n", t.TaskName)
	// if t.MessageChan != nil {
	// 	close(t.MessageChan)
	// }

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// MessageProducer 消息生产者
type MessageProducer struct {
	task.Task
	ProducerName string
	MessageChan  chan string
}

func (p *MessageProducer) Start() error {
	// TODO: 取消下面的注释来完成任务启动逻辑
	// p.Info("消息生产者启动", "producerName", p.ProducerName)
	// fmt.Printf("消息生产者 %s 已启动\n", p.ProducerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Start方法中的TODO注释")
	return fmt.Errorf("Start方法未完成")
}

func (p *MessageProducer) Run() error {
	// TODO: 取消下面的注释来完成任务运行逻辑
	// p.Info("消息生产者运行中", "producerName", p.ProducerName)
	// fmt.Printf("消息生产者 %s 正在运行...\n", p.ProducerName)
	//
	// // 发送一些消息
	// messages := []string{
	// 	"Hello World",
	// 	"GoTask is awesome",
	// 	"Channel communication",
	// 	"Task coordination",
	// }
	//
	// for i, msg := range messages {
	// 	time.Sleep(500 * time.Millisecond)
	// 	fmt.Printf("消息生产者 %s 发送消息 %d: %s\n", p.ProducerName, i+1, msg)
	// 	p.MessageChan <- msg
	// }
	//
	// fmt.Printf("消息生产者 %s 完成消息发送\n", p.ProducerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Run方法中的TODO注释")
	return fmt.Errorf("Run方法未完成")
}

func (p *MessageProducer) Dispose() {
	// TODO: 取消下面的注释来完成任务清理逻辑
	// p.Info("消息生产者清理", "producerName", p.ProducerName)
	// fmt.Printf("消息生产者 %s 已清理\n", p.ProducerName)

	// 验证：检查是否完成了TODO
	fmt.Println("❌ 错误：请取消Dispose方法中的TODO注释")
}

// 使用RootManager作为根任务管理器
type TaskManager = task.RootManager[uint32, *MessageTask]

func main() {
	fmt.Println("=== GoTask Lesson 4: ChannelTask通道任务 ===")
	fmt.Println("本课程将教你如何使用ChannelTask进行任务间通信")
	fmt.Println("请按照TODO注释的提示，逐步取消注释来完成代码")
	fmt.Println()

	// 创建根任务管理器
	root := &TaskManager{}
	root.Init()

	// 创建消息通道
	messageChan := make(chan string, 10)

	// 创建消息处理任务
	messageTask := &MessageTask{
		TaskName:    "消息处理器",
		MessageChan: messageChan,
	}

	// 将消息任务添加到根管理器中
	root.AddTask(messageTask)

	// 等待消息任务启动
	err := messageTask.WaitStarted()
	if err != nil {
		fmt.Printf("❌ 消息任务启动失败: %v\n", err)
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 创建消息生产者
	producer := &MessageProducer{
		ProducerName: "消息生产者",
		MessageChan:  messageChan,
	}

	// TODO: 取消下面的注释来添加消息生产者到根管理器中
	// root.AddTask(producer)

	// 验证：检查是否添加了生产者
	if producer.GetTaskID() == 0 {
		fmt.Println("❌ 错误：请取消AddTask的TODO注释")
		fmt.Println("请检查并完成所有TODO注释")
		root.Shutdown()
		return
	}

	// 等待所有任务完成
	producer.WaitStopped()
	messageTask.Stop(task.ErrTaskComplete)
	messageTask.WaitStopped()

	fmt.Println("=== 课程完成 ===")
	fmt.Println("如果看到消息发送和接收的日志，说明你已经成功完成了Lesson 4!")

	// 优雅关闭
	root.Shutdown()
}
