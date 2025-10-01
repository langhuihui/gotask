package lessons

// 为了避免类型重复声明，这里使用不同的类型名

import (
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

type WorkerTask02_1 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_1) Start() error {
	t.Info("工作线程启动", "workerID", t.WorkerID)
	return nil
}

func (t *WorkerTask02_1) Run() error {
	t.Info("工作线程运行", "workerID", t.WorkerID)
	return nil
}

// ManagerJob 管理任务容器
type ManagerJob struct {
	task.Job
	JobName string
}

// TestLesson02 测试Job容器管理
func TestLesson02_1(t *testing.T) {
	t.Log("=== Lesson 2-1: Job容器管理 ===")
	t.Log("课程目标：学习如何使用Job来管理多个子任务，了解任务层次结构")
	t.Log("核心概念：Job容器可以包含多个子任务，管理父子任务的生命周期关系")
	t.Log("重要特性：当所有子任务完成后，Job会自动停止并进入Disposed状态")
	t.Log("学习内容：AddTask添加子任务、任务层次结构管理、WaitStarted/WaitStopped方法")

	// 创建管理任务
	manager := &ManagerJob{JobName: "工作管理器"}

	// 将管理任务添加到根管理器中（重要）
	root.AddTask(manager)

	// 创建多个工作线程
	workers := make([]*WorkerTask02_1, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_1{WorkerID: i}
		manager.AddTask(workers[i-1])
	}

	// 等待所有任务完成（子任务完成后 Job 会自动停止）,TODO: 取消注释来完成任务管理
	// manager.WaitStopped()

	if manager.GetState() == task.TASK_STATE_DISPOSED {
		t.Log("Lesson 2-1 测试通过：Job容器管理")
		return
	}
	t.Errorf("课程未通过")
}

type WorkerTask02_2 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_2) Start() error {
	t.Info("工作线程启动", "workerID", t.WorkerID)
	return nil
}

// TODO: 取消注释来完成任务运行
// func (t *WorkerTask02_2) Run() error {
// 	t.Info("工作线程运行", "workerID", t.WorkerID)
// 	return nil
// }

func TestLesson02_2(t *testing.T) {
	t.Log("=== Lesson 2-2: 任务生命周期 - 没有Run方法的任务 ===")
	t.Log("课程目标：理解任务的Run方法对任务生命周期的影响")
	t.Log("核心概念：没有Run方法的任务会在Start后保持运行状态，不会自动结束")
	t.Log("学习内容：任务状态管理、长期运行任务的特性、Job容器的停止条件")

	// 创建管理任务
	manager := &ManagerJob{JobName: "工作管理器"}

	// 将管理任务添加到根管理器中（重要）
	root.AddTask(manager)

	// 创建多个工作线程
	workers := make([]*WorkerTask02_2, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_2{WorkerID: i}
		manager.AddTask(workers[i-1])
	}
	time.AfterFunc(1*time.Second, func() {
		if manager.GetState() == task.TASK_STATE_DISPOSED {
			t.Log("Lesson 2-2 测试通过：Job容器管理")
			return
		}
		t.Errorf("课程未通过")
	})
	manager.WaitStopped()
}

type WorkerTask02_3 struct {
	task.Task
	WorkerID int
}

func (t *WorkerTask02_3) Start() error {
	t.Info("工作线程启动", "workerID", t.WorkerID)
	return nil
}

// TestLesson02_3 测试Job的Stop方法会导致所有子任务被Stop
func TestLesson02_3(t *testing.T) {
	t.Log("=== Lesson 2-3: Job的Stop传播机制 ===")
	t.Log("课程目标：理解Job的Stop方法对子任务的影响")
	t.Log("核心概念：调用Job的Stop方法会导致所有子任务被Stop")
	t.Log("学习内容：Job的停止传播、子任务生命周期管理")

	// 创建管理任务
	manager := &ManagerJob{JobName: "工作管理器"}

	// 将管理任务添加到根管理器中（重要）
	root.AddTask(manager)

	// 创建多个工作线程（没有Run方法，会一直运行）
	workers := make([]*WorkerTask02_3, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_3{WorkerID: i}
		manager.AddTask(workers[i-1])
		workers[i-1].WaitStarted()
	}

	// 主动停止管理任务，TODO: 取消注释来完成任务停止
	t.Log("主动停止 Job 任务...")
	// manager.Stop(task.ErrStopByUser)

	time.Sleep(1 * time.Second)

	// 验证所有子任务都已经被停止
	allStopped := true
	for _, worker := range workers {
		if worker.GetState() != task.TASK_STATE_DISPOSED {
			t.Errorf("工作线程 %d 未被停止，状态: %d", worker.WorkerID, worker.GetState())
			allStopped = false
		}
	}

	// 验证管理任务本身也已停止
	if manager.GetState() != task.TASK_STATE_DISPOSED {
		t.Errorf("管理任务未被停止，状态: %d", manager.GetState())
		allStopped = false
	}

	if allStopped {
		t.Log("✓ Lesson 2-3 测试通过：Job的Stop方法成功停止了所有子任务")
	} else {
		t.Errorf("✗ Lesson 2-3 测试失败：部分任务未被正确停止")
	}
}

type WorkerTask02_4 struct {
	task.Task
	WorkerID  int
	StartTime time.Time
}

func (t *WorkerTask02_4) Start() error {
	t.StartTime = time.Now()
	t.Info("工作线程启动", "workerID", t.WorkerID, "time", t.StartTime)
	return nil
}

// TODO：第二种办法，使用Go方法代替Run实现异步执行
func (t *WorkerTask02_4) Run() error {
	// 第一个任务阻塞2秒，其他任务快速完成
	if t.WorkerID == 1 {
		t.Info("工作线程1开始阻塞运行", "workerID", t.WorkerID)
		time.Sleep(2 * time.Second) // TODO: 试试注释掉这一行，观察任务启动时间的变化
		t.Info("工作线程1完成运行", "workerID", t.WorkerID)
	} else {
		t.Info("工作线程运行", "workerID", t.WorkerID)
	}
	return nil
}

// TestLesson02_4 测试子任务的Run会阻塞其他子任务的运行
func TestLesson02_4(t *testing.T) {
	t.Log("=== Lesson 2-4: 子任务的Run阻塞特性 ===")
	t.Log("课程目标：理解子任务的Run方法会阻塞事件循环，影响其他子任务的启动")
	t.Log("核心概念：Job的事件循环是单线程的，子任务的Start和Run方法会同步执行")
	t.Log("")
	t.Log("📝 实验步骤：")
	t.Log("   1. 运行测试，观察工作线程的启动时间")
	t.Log("   2. 注释掉第183行的 time.Sleep，再次运行")
	t.Log("   3. 对比两次运行的时间差，理解Run方法的阻塞特性")

	// 创建管理任务
	manager := &ManagerJob{JobName: "工作管理器"}
	root.AddTask(manager)

	// 创建多个工作线程
	workers := make([]*WorkerTask02_4, 3)
	for i := 1; i <= 3; i++ {
		workers[i-1] = &WorkerTask02_4{WorkerID: i}
		manager.AddTask(workers[i-1])
	}

	// 使用Timer检查第三个子任务的状态
	t.Log("")
	t.Log("🔍 使用Timer检查第三个子任务的状态：")

	time.Sleep(1 * time.Second)
	// 1秒后检查第三个任务的状态
	worker3State := workers[2].GetState()
	t.Logf("  1秒后工作线程3的状态: %d", worker3State)

	if worker3State < task.TASK_STATE_STARTED {
		t.Log("  ✓ 验证通过：工作线程3在1秒后还未启动")
		t.Log("    说明：工作线程1的Run方法确实阻塞了事件循环")
		t.Log("    结论：Run方法是同步执行的，会阻塞后续任务")
		t.Log("课程未通过")
	} else {
		t.Log("✓ Lesson 2-4 测试通过：Job的Run方法会阻塞后续任务")
	}
}
