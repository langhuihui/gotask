package task

import (
	"runtime"
	"strconv"
)

// goID 返回当前 goroutine 的 id。
//
// 纯 Go 实现:解析 runtime.Stack 首行 "goroutine <id> [..." 的数字,无需汇编/cgo,
// 跨平台一致。仅用于事件循环的重入检测(Job.Call 的入队路径上调用一次),频率不高,
// 这点开销可忽略。返回 0 表示解析失败(调用方应将其视为"非循环 goroutine")。
func goID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	const prefix = "goroutine "
	if n < len(prefix) {
		return 0
	}
	b := buf[len(prefix):n]
	i := 0
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	id, err := strconv.ParseInt(string(b[:i]), 10, 64)
	if err != nil {
		return 0
	}
	return id
}
