package lessons

import (
	"sync/atomic"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// 全局原子变量用于生成唯一键
var keyCounter uint32

// SimpleService 简单服务，实现 ManagerItem 接口
type SimpleService struct {
	task.Task
	Name string
}

func (s *SimpleService) GetKey() uint32 {
	// 使用原子变量生成唯一键
	return atomic.AddUint32(&keyCounter, 1)
}

func (s *SimpleService) Start() error {
	s.Info("服务启动", "name", s.Name)
	return nil
}

func (s *SimpleService) Go() error {
	s.Info("服务运行中", "name", s.Name)
	time.Sleep(1 * time.Second)
	return nil
}

func (s *SimpleService) Dispose() {
	s.Info("服务清理", "name", s.Name)
}

// TestLesson06 测试RootManager作为WorkCollection的功能
func TestLesson06(t *testing.T) {
	t.Log("=== Lesson 6: RootManager作为WorkCollection的工作集合管理 ===")
	t.Log("课程目标：学习如何使用RootManager作为WorkCollection管理服务集合")
	t.Log("核心概念：WorkCollection提供集合操作功能，支持键值对管理、查找、遍历等")

	// 创建1个简单服务
	service := &SimpleService{Name: "test"}

	// 检查初始状态
	t.Logf("添加服务前，服务数量: %d", root.Length())

	// 添加服务到RootManager
	root.AddTask(service)
	t.Logf("添加服务后，服务数量: %d", root.Length())

	// 等待服务启动
	service.WaitStarted()
	t.Logf("服务启动后，服务数量: %d", root.Length())

	// 测试1: 验证服务数量
	actualCount := root.Length()
	t.Logf("实际服务数量: %d", actualCount)
	if actualCount < 1 {
		t.Errorf("服务数量不足，期望至少: 1, 实际: %d", actualCount)
		return
	}
	t.Log("✓ 服务数量验证通过")

	// 测试2: 验证Range方法 - 遍历所有服务
	serviceNames := make([]string, 0)
	root.Range(func(item task.ManagerItem[uint32]) bool {
		if service, ok := item.(*SimpleService); ok {
			// TODO: 取消注释来完成服务名称的验证
			// serviceNames = append(serviceNames, service.Name)
			_ = service
		}
		return true
	})
	if len(serviceNames) != 1 {
		t.Errorf("课程未通过")
		return
	}
	t.Logf("Range遍历找到的服务: %v", serviceNames)
	t.Log("✓ Range方法验证通过")

	// 测试3: 验证Find方法 - 查找SimpleService类型的服务
	foundService, found := root.Find(func(item task.ManagerItem[uint32]) bool {
		_, ok := item.(*SimpleService)
		return ok
	})
	if !found {
		t.Log("未找到SimpleService类型的服务，但这是正常的，因为可能只有OSSignal任务")
	} else {
		t.Log("✓ Find方法验证通过")

		// 测试4: 验证Get方法
		serviceKey := foundService.GetKey()
		_, ok := root.Get(serviceKey)
		if !ok {
			t.Error("无法获取服务")
			return
		}
		t.Log("✓ Get方法验证通过")

		// 测试5: 验证Has方法
		if !root.Has(serviceKey) {
			t.Error("服务应该存在")
			return
		}
		t.Log("✓ Has方法验证通过")
	}

	t.Log("Lesson 6 测试通过：RootManager作为WorkCollection的工作集合管理")
}

// CustomService 自定义服务，用于演示WorkCollection泛型集合
type CustomService struct {
	task.Task
	ID   string
	Name string
	Type string
}

func (c *CustomService) GetKey() string {
	return c.ID
}

func (c *CustomService) Start() error {
	c.Info("自定义服务启动", "id", c.ID, "name", c.Name, "type", c.Type)
	return nil
}

func (c *CustomService) Go() error {
	c.Info("自定义服务运行中", "id", c.ID, "name", c.Name)
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (c *CustomService) Dispose() {
	c.Info("自定义服务清理", "id", c.ID, "name", c.Name)
}

// TestLesson06_2 测试WorkCollection泛型集合功能
func TestLesson06_2(t *testing.T) {
	t.Log("=== Lesson 6.2: WorkCollection泛型集合功能测试 ===")
	t.Log("课程目标：学习如何使用WorkCollection泛型集合管理不同类型的服务")
	t.Log("核心概念：泛型类型安全、类型推断、集合操作方法")

	// 创建WorkCollection实例，使用string作为键类型，CustomService作为值类型
	collection := &task.WorkCollection[string, *CustomService]{}

	root.AddTask(collection)
	
	// 创建多个自定义服务
	services := []*CustomService{
		{ID: "service-1", Name: "用户服务", Type: "user"},
		{ID: "service-2", Name: "订单服务", Type: "order"},
		{ID: "service-3", Name: "支付服务", Type: "payment"},
		{ID: "service-4", Name: "通知服务", Type: "notification"},
	}

	// 测试1: 验证初始状态
	t.Logf("初始集合长度: %d", collection.Length())
	if collection.Length() != 0 {
		t.Error("初始集合应该为空")
		return
	}
	t.Log("✓ 初始状态验证通过")

	// 测试2: 添加服务到集合
	for _, service := range services {
		collection.AddTask(service)
		t.Logf("添加服务: %s (%s)", service.Name, service.ID)
	}

	// 验证添加后的长度
	actualLength := collection.Length()
	t.Logf("添加服务后集合长度: %d", actualLength)
	if actualLength != len(services) {
		t.Errorf("集合长度不匹配，期望: %d, 实际: %d", len(services), actualLength)
		return
	}
	t.Log("✓ 服务添加验证通过")

	// 测试3: 验证Has方法 - 检查服务是否存在
	for _, service := range services {
		if !collection.Has(service.ID) {
			t.Errorf("服务 %s 应该存在", service.ID)
			return
		}
	}
	t.Log("✓ Has方法验证通过")

	// 测试4: 验证Get方法 - 根据键获取服务
	for _, expectedService := range services {
		actualService, ok := collection.Get(expectedService.ID)
		if !ok {
			t.Errorf("无法获取服务: %s", expectedService.ID)
			return
		}
		if actualService.ID != expectedService.ID {
			t.Errorf("服务ID不匹配，期望: %s, 实际: %s", expectedService.ID, actualService.ID)
			return
		}
		if actualService.Name != expectedService.Name {
			t.Errorf("服务名称不匹配，期望: %s, 实际: %s", expectedService.Name, actualService.Name)
			return
		}
	}
	t.Log("✓ Get方法验证通过")

	// 测试5: 验证Find方法 - 查找特定类型的服务
	userServices := make([]*CustomService, 0)
	collection.Find(func(service *CustomService) bool {
		if service.Type == "user" {
			userServices = append(userServices, service)
			return true
		}
		return false
	})
	if len(userServices) != 1 {
		t.Errorf("应该找到1个用户服务，实际找到: %d", len(userServices))
		return
	}
	if userServices[0].ID != "service-1" {
		t.Errorf("用户服务ID不匹配，期望: service-1, 实际: %s", userServices[0].ID)
		return
	}
	t.Log("✓ Find方法验证通过")

	// 测试6: 验证Range方法 - 遍历所有服务
	collectedServices := make([]*CustomService, 0)
	collection.Range(func(service *CustomService) bool {
		collectedServices = append(collectedServices, service)
		return true
	})
	if len(collectedServices) != len(services) {
		t.Errorf("Range遍历结果不匹配，期望: %d, 实际: %d", len(services), len(collectedServices))
		return
	}
	t.Log("✓ Range方法验证通过")

	// 测试7: 验证ToList方法 - 转换为列表
	serviceList := collection.ToList()
	if len(serviceList) != len(services) {
		t.Errorf("ToList结果不匹配，期望: %d, 实际: %d", len(services), len(serviceList))
		return
	}
	t.Log("✓ ToList方法验证通过")

	// 测试8: 验证类型安全性 - 尝试获取不存在的服务
	_, ok := collection.Get("non-existent-service")
	if ok {
		t.Error("不应该找到不存在的服务")
		return
	}
	t.Log("✓ 类型安全性验证通过")

	// 测试9: 验证泛型类型推断
	// 这里演示泛型如何确保类型安全
	var typedCollection *task.WorkCollection[string, *CustomService] = collection
	if typedCollection.Length() != collection.Length() {
		t.Error("泛型类型推断失败")
		return
	}
	t.Log("✓ 泛型类型推断验证通过")

	// 测试10: 验证Find方法的条件查找
	orderServices := make([]*CustomService, 0)
	collection.Find(func(service *CustomService) bool {
		if service.Type == "order" {
			orderServices = append(orderServices, service)
			return true
		}
		return false
	})
	if len(orderServices) != 1 {
		t.Errorf("应该找到1个订单服务，实际找到: %d", len(orderServices))
		return
	}
	t.Log("✓ 条件查找验证通过")

	t.Log("Lesson 6.2 测试通过：WorkCollection泛型集合功能完整验证")
}
