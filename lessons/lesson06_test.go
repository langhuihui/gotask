package lessons

import (
	"sync/atomic"
	"testing"
	"time"

	task "github.com/langhuihui/gotask"
)

// Global atomic variable for generating unique keys
var keyCounter uint32

// SimpleService Simple service, implementing ManagerItem interface
type SimpleService struct {
	task.Task
	Name string
}

func (s *SimpleService) GetKey() uint32 {
	// Use atomic variable to generate unique key
	return atomic.AddUint32(&keyCounter, 1)
}

func (s *SimpleService) Start() error {
	s.Info("Service started", "name", s.Name)
	return nil
}

func (s *SimpleService) Go() error {
	s.Info("Service running", "name", s.Name)
	time.Sleep(1 * time.Second)
	return nil
}

func (s *SimpleService) Dispose() {
	s.Info("Service cleaned up", "name", s.Name)
}

// TestLesson06 Test RootManager as WorkCollection functionality
func TestLesson06(t *testing.T) {
	t.Log("=== Lesson 6: RootManager as WorkCollection Work Set Management ===")
	t.Log("Course Objective: Learn how to use RootManager as WorkCollection to manage service sets")
	t.Log("Core Concepts: WorkCollection provides collection operations, supports key-value management, search, iteration, etc.")

	// Create 1 simple service
	service := &SimpleService{Name: "test"}

	// Check initial state
	t.Logf("Before adding service, service count: %d", root.Length())

	// Add service to RootManager
	root.AddTask(service)
	t.Logf("After adding service, service count: %d", root.Length())

	// Wait for service to start
	service.WaitStarted()
	t.Logf("After service started, service count: %d", root.Length())

	// Test 1: Verify service count
	actualCount := root.Length()
	t.Logf("Actual service count: %d", actualCount)
	if actualCount < 1 {
		t.Errorf("Insufficient services, expected at least: 1, actual: %d", actualCount)
		return
	}
	t.Log("✓ Service count verification passed")

	// Test 2: Verify Range method - iterate through all services
	serviceNames := make([]string, 0)
	root.Range(func(item task.ManagerItem[uint32]) bool {
		if service, ok := item.(*SimpleService); ok {
			// TODO: Uncomment to complete service name verification
			// serviceNames = append(serviceNames, service.Name)
			_ = service
		}
		return true
	})
	if len(serviceNames) != 1 {
		t.Errorf("Course not passed")
		return
	}
	t.Logf("Services found by Range iteration: %v", serviceNames)
	t.Log("✓ Range method verification passed")

	// Test 3: Verify Find method - find services of SimpleService type
	foundService, found := root.Find(func(item task.ManagerItem[uint32]) bool {
		_, ok := item.(*SimpleService)
		return ok
	})
	if !found {
		t.Log("SimpleService type service not found, but this is normal as there might only be OSSignal tasks")
	} else {
		t.Log("✓ Find method verification passed")

		// Test 4: Verify Get method
		serviceKey := foundService.GetKey()
		_, ok := root.Get(serviceKey)
		if !ok {
			t.Error("Unable to get service")
			return
		}
		t.Log("✓ Get method verification passed")

		// Test 5: Verify Has method
		if !root.Has(serviceKey) {
			t.Error("Service should exist")
			return
		}
		t.Log("✓ Has method verification passed")
	}

	t.Log("Lesson 6 test passed: RootManager as WorkCollection work set management")
}

// CustomService Custom service, demonstrating WorkCollection generic collection
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
	c.Info("Custom service started", "id", c.ID, "name", c.Name, "type", c.Type)
	return nil
}

func (c *CustomService) Go() error {
	c.Info("Custom service running", "id", c.ID, "name", c.Name)
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (c *CustomService) Dispose() {
	c.Info("Custom service cleaned up", "id", c.ID, "name", c.Name)
}

// TestLesson06_2 Test WorkCollection generic collection functionality
func TestLesson06_2(t *testing.T) {
	t.Log("=== Lesson 6.2: WorkCollection Generic Collection Functionality Test ===")
	t.Log("Course Objective: Learn how to use WorkCollection generic collection to manage different types of services")
	t.Log("Core Concepts: Generic type safety, type inference, collection operation methods")

	// Create WorkCollection instance, using string as key type, CustomService as value type
	collection := &task.WorkCollection[string, *CustomService]{}

	root.AddTask(collection)

	// Create multiple custom services
	services := []*CustomService{
		{ID: "service-1", Name: "User Service", Type: "user"},
		{ID: "service-2", Name: "Order Service", Type: "order"},
		{ID: "service-3", Name: "Payment Service", Type: "payment"},
		{ID: "service-4", Name: "Notification Service", Type: "notification"},
	}

	// Test 1: Verify initial state
	t.Logf("Initial collection length: %d", collection.Length())
	if collection.Length() != 0 {
		t.Error("Initial collection should be empty")
		return
	}
	t.Log("✓ Initial state verification passed")

	// Test 2: Add services to collection
	for _, service := range services {
		collection.AddTask(service)
		t.Logf("Added service: %s (%s)", service.Name, service.ID)
	}

	// Verify length after adding
	actualLength := collection.Length()
	t.Logf("Collection length after adding services: %d", actualLength)
	if actualLength != len(services) {
		t.Errorf("Collection length mismatch, expected: %d, actual: %d", len(services), actualLength)
		return
	}
	t.Log("✓ Service addition verification passed")

	// Test 3: Verify Has method - check if service exists
	for _, service := range services {
		if !collection.Has(service.ID) {
			t.Errorf("Service %s should exist", service.ID)
			return
		}
	}
	t.Log("✓ Has method verification passed")

	// Test 4: Verify Get method - get service by key
	for _, expectedService := range services {
		actualService, ok := collection.Get(expectedService.ID)
		if !ok {
			t.Errorf("Unable to get service: %s", expectedService.ID)
			return
		}
		if actualService.ID != expectedService.ID {
			t.Errorf("Service ID mismatch, expected: %s, actual: %s", expectedService.ID, actualService.ID)
			return
		}
		if actualService.Name != expectedService.Name {
			t.Errorf("Service name mismatch, expected: %s, actual: %s", expectedService.Name, actualService.Name)
			return
		}
	}
	t.Log("✓ Get method verification passed")

	// Test 5: Verify Find method - find specific type of services
	userServices := make([]*CustomService, 0)
	collection.Find(func(service *CustomService) bool {
		if service.Type == "user" {
			userServices = append(userServices, service)
			return true
		}
		return false
	})
	if len(userServices) != 1 {
		t.Errorf("Should find 1 user service, actually found: %d", len(userServices))
		return
	}
	if userServices[0].ID != "service-1" {
		t.Errorf("User service ID mismatch, expected: service-1, actual: %s", userServices[0].ID)
		return
	}
	t.Log("✓ Find method verification passed")

	// Test 6: Verify Range method - iterate through all services
	collectedServices := make([]*CustomService, 0)
	collection.Range(func(service *CustomService) bool {
		collectedServices = append(collectedServices, service)
		return true
	})
	if len(collectedServices) != len(services) {
		t.Errorf("Range iteration result mismatch, expected: %d, actual: %d", len(services), len(collectedServices))
		return
	}
	t.Log("✓ Range method verification passed")

	// Test 7: Verify ToList method - convert to list
	serviceList := collection.ToList()
	if len(serviceList) != len(services) {
		t.Errorf("ToList result mismatch, expected: %d, actual: %d", len(services), len(serviceList))
		return
	}
	t.Log("✓ ToList method verification passed")

	// Test 8: Verify type safety - try to get non-existent service
	_, ok := collection.Get("non-existent-service")
	if ok {
		t.Error("Should not find non-existent service")
		return
	}
	t.Log("✓ Type safety verification passed")

	// Test 9: Verify generic type inference
	// Demonstrate how generics ensure type safety
	var typedCollection *task.WorkCollection[string, *CustomService] = collection
	if typedCollection.Length() != collection.Length() {
		t.Error("Generic type inference failed")
		return
	}
	t.Log("✓ Generic type inference verification passed")

	// Test 10: Verify Find method's conditional search
	orderServices := make([]*CustomService, 0)
	collection.Find(func(service *CustomService) bool {
		if service.Type == "order" {
			orderServices = append(orderServices, service)
			return true
		}
		return false
	})
	if len(orderServices) != 1 {
		t.Errorf("Should find 1 order service, actually found: %d", len(orderServices))
		return
	}
	t.Log("✓ Conditional search verification passed")

	t.Log("Lesson 6.2 test passed: WorkCollection generic collection functionality fully verified")
}
