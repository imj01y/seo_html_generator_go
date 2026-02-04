package pool

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unsafe"
)

// StringMemorySize 计算字符串占用的内存大小(字节)
func StringMemorySize(s string) int64 {
	// string header (16 bytes) + data length
	return 16 + int64(len(s))
}

// SliceMemorySize 计算字符串切片占用的内存大小(字节)
func SliceMemorySize(items []string) int64 {
	if len(items) == 0 {
		return 0
	}
	// slice header (24 bytes) + sum of string sizes
	size := int64(unsafe.Sizeof(items))
	for _, item := range items {
		size += StringMemorySize(item)
	}
	return size
}

// encodeText 将文本中的非ASCII字符编码为HTML实体
// 这是 HTMLEntityEncoder.EncodeText 的简化版本
func encodeText(text string) string {
	if text == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(text) * 2) // 预分配空间

	mixRatio := 0.5 // 50% hex, 50% decimal
	for _, r := range text {
		if r <= 127 {
			// ASCII字符,保持原样
			sb.WriteRune(r)
		} else {
			// 非ASCII字符,编码
			if rand.Float64() < mixRatio {
				// 十六进制编码: &#x数字;
				sb.WriteString(fmt.Sprintf("&#x%x;", r))
			} else {
				// 十进制编码: &#数字;
				sb.WriteString(fmt.Sprintf("&#%d;", r))
			}
		}
	}

	return sb.String()
}
