package task

import (
	"context"
	"log/slog"
)

// Manager 泛型管理器
type Manager[K comparable, T ManagerItem[K]] struct {
	WorkCollection[K, T]
}

// NewManager 创建新的管理器
func NewManager[K comparable, T ManagerItem[K]]() *Manager[K, T] {
	m := &Manager[K, T]{}
	m.Context = context.Background()
	m.handler = m
	m.Logger = slog.Default()
	return m
}