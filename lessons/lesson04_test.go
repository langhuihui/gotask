//go:build lesson

package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// MessageTask Message processing task
type MessageTask struct {
	task.ChannelTask
	ProcessedCount int
}

func (t *MessageTask) Tick(signal any) {
	if msg, ok := signal.(string); ok {
		t.ProcessMessage(msg)
	}
}

// ProcessMessage Process received message
func (t *MessageTask) ProcessMessage(msg string) {
	// Mark message as processed
	t.ProcessedCount++
}

// ProcessedCount Record processed message count
func (t *MessageTask) GetProcessedCount() int {
	return t.ProcessedCount
}

// MessageProducer Message producer
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

// TestLesson04 Test ChannelTask communication tasks
func TestLesson04(t *testing.T) {
	t.Log("=== Lesson 4: ChannelTask Communication Tasks ===")
	t.Skip("exercise: complete the TODO below (then delete this line) to check your work")
	t.Log("Learning Objective: Understand ChannelTask's message processing mechanism")
	t.Log("Task: Uncomment t.ProcessMessage(msg) in MessageTask.Tick() and delete _ = msg")
	t.Log("If code not modified, test will fail!")
	messageChan := make(chan string, 10)
	// Create message processing task
	messageTask := &MessageTask{}
	// TODO: Student needs to uncomment below code to correctly process messages
	// messageTask.SignalChan = messageChan

	root.AddTask(messageTask)
	messageTask.WaitStarted()

	// Create message producer
	producer := &MessageProducer{
		MessageChan: messageChan,
	}
	root.AddTask(producer)

	// Wait for producer to complete
	producer.WaitStopped()
	time.Sleep(500 * time.Millisecond)

	// Verification: Check if messages were processed
	processedCount := messageTask.GetProcessedCount()
	if processedCount == 0 {
		t.Fatal("Course not passed")
	}

	if processedCount != 3 {
		t.Fatalf("Expected to process 3 messages, actually processed %d", processedCount)
	}

	t.Logf("Success! Processed %d messages", processedCount)

	t.Log("Lesson 4 completed! Channel communication working normally")
}
