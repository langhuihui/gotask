package lessons

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// TimerTask 定时任务
type TimerTask struct {
	task.TickTask
	TaskName string
	Counter  int
}

func (t *TimerTask) GetTickInterval() time.Duration {
	return 1 * time.Second
}

func (t *TimerTask) Start() (err error) {
	t.Info("定时任务启动", "taskName", t.TaskName)
	// TODO: 取消注释下面的代码来正确执行定时任务
	// err = t.TickTask.Start()
	return
}

func (t *TimerTask) Tick(tick any) {
	t.Counter++

	// 执行5次后自动停止
	if t.Counter >= 3 {
		t.Info("定时任务执行完成，自动停止", "taskName", t.TaskName)
		t.Stop(task.ErrTaskComplete)
	}
}

// TestLesson05 测试TickTask定时任务
func TestLesson05(t *testing.T) {
	t.Log("=== Lesson 5: TickTask定时任务 ===")
	t.Log("学习目标：理解TickTask的定时执行机制")
	t.Log("任务：在TimerTask.Start()中取消注释相关代码来实现定时任务逻辑")

	// 创建定时任务
	timerTask := &TimerTask{TaskName: "计数器任务"}

	root.AddTask(timerTask)

	time.Sleep(4 * time.Second)

	// 验证：检查是否执行了预期的次数
	if timerTask.Counter != 3 {
		t.Fatalf("期望执行5次，实际执行了%d次", timerTask.Counter)
	}

	t.Logf("成功！定时任务执行了%d次", timerTask.Counter)
	t.Log("Lesson 3 完成！定时任务正常工作")
}
