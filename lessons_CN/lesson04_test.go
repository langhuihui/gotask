package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// MessageTask 消息处理任务
type MessageTask struct {
	task.ChannelTask
	ProcessedCount int
}

func (t *MessageTask) Tick(signal any) {
	if msg, ok := signal.(string); ok {
		t.ProcessMessage(msg)
	}
}

// ProcessMessage 处理接收到的消息
func (t *MessageTask) ProcessMessage(msg string) {
	// 标记消息已处理
	t.ProcessedCount++
}

// ProcessedCount 记录处理的消息数量
func (t *MessageTask) GetProcessedCount() int {
	return t.ProcessedCount
}

// MessageProducer 消息生产者
type MessageProducer struct {
	task.Task
	MessageChan chan string
}

func (p *MessageProducer) Run() error {
	messages := []string{"Hello", "World", "GoTask"}
	for _, msg := range messages {
		time.Sleep(100 * time.Millisecond)
		p.MessageChan <- msg
	}
	return nil
}

// TestLesson04 测试ChannelTask通道任务
func TestLesson04(t *testing.T) {
	t.Log("=== Lesson 4: ChannelTask通道任务 ===")
	t.Log("学习目标：理解ChannelTask的消息处理机制")
	t.Log("任务：在MessageTask.Tick()中取消注释 t.ProcessMessage(msg) 并删除 _ = msg")
	t.Log("如果不修改代码，测试将失败！")
	messageChan := make(chan string, 10)
	// 创建消息处理任务
	messageTask := &MessageTask{}
	// TODO: 学员需要取消注释下面的代码来正确处理消息
	// messageTask.SignalChan = messageChan

	root.AddTask(messageTask)
	messageTask.WaitStarted()

	// 创建消息生产者
	producer := &MessageProducer{
		MessageChan: messageChan,
	}
	root.AddTask(producer)

	// 等待生产者完成
	producer.WaitStopped()
	time.Sleep(500 * time.Millisecond)

	// 验证：检查是否处理了消息
	processedCount := messageTask.GetProcessedCount()
	if processedCount == 0 {
		t.Fatal("课程未通过")
	}

	if processedCount != 3 {
		t.Fatalf("期望处理3条消息，实际处理了%d条", processedCount)
	}

	t.Logf("成功！处理了%d条消息", processedCount)

	t.Log("Lesson 4 完成！通道通信正常工作")
}
