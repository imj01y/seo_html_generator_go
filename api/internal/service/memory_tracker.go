// api/internal/service/memory_tracker.go
package core

import "sync/atomic"

// StringMemorySize 计算字符串的内存占用
// Go 中 string = 16 字节 header + len(s) 字节内容
const StringHeaderSize = 16

// StringMemorySize 返回字符串的内存占用字节数
func StringMemorySize(s string) int64 {
	return int64(StringHeaderSize + len(s))
}

// SliceMemorySize 计算字符串切片的总内存占用
func SliceMemorySize(ss []string) int64 {
	var total int64
	for _, s := range ss {
		total += StringMemorySize(s)
	}
	return total
}

// MemoryTracker 内存追踪器（线程安全）
type MemoryTracker struct {
	bytes atomic.Int64
}

// Add 增加字符串的内存占用
func (t *MemoryTracker) Add(s string) {
	t.bytes.Add(StringMemorySize(s))
}

// AddBytes 直接增加字节数
func (t *MemoryTracker) AddBytes(n int64) {
	t.bytes.Add(n)
}

// Remove 减少字符串的内存占用
func (t *MemoryTracker) Remove(s string) {
	t.bytes.Add(-StringMemorySize(s))
}

// RemoveBytes 直接减少字节数
func (t *MemoryTracker) RemoveBytes(n int64) {
	t.bytes.Add(-n)
}

// Reset 重置为 0
func (t *MemoryTracker) Reset() {
	t.bytes.Store(0)
}

// Set 设置为指定值
func (t *MemoryTracker) Set(n int64) {
	t.bytes.Store(n)
}

// Bytes 获取当前内存占用
func (t *MemoryTracker) Bytes() int64 {
	return t.bytes.Load()
}

// FormatBytes 格式化字节数为人类可读格式
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return memFormatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return memFormatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return memFormatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return memItoa(bytes) + " B"
	}
}

func memFormatFloat(f float64) string {
	if f >= 100 {
		return memItoa(int64(f))
	}
	// 保留两位小数
	intPart := int64(f)
	fracPart := int64((f - float64(intPart)) * 100)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	if fracPart == 0 {
		return memItoa(intPart)
	}
	return memItoa(intPart) + "." + memPadZero(fracPart, 2)
}

func memItoa(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func memPadZero(n int64, width int) string {
	s := memItoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
