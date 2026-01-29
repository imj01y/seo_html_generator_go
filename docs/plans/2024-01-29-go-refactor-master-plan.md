# SEO HTML Generator Go 重构主计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 Python 后端重构为 Go，提升 SEO 页面渲染性能，实现设计文档中描述的所有功能。

**Architecture:**
- Go 服务处理 SEO 渲染、管理 API、缓存管理、定时任务调度
- Python 服务仅处理数据爬取和数据处理
- 通过 HTTP API + Redis 队列通信

**Tech Stack:** Go 1.21+, Gin, MySQL, Redis, zerolog, robfig/cron

---

## 重构阶段概览

本重构分为 8 个阶段，每个阶段独立可测试：

| 阶段 | 模块 | 优先级 | 依赖 |
|------|------|--------|------|
| 1 | 爬虫检测配置化 | P0 | 无 |
| 2 | 模板分析器 | P0 | 无 |
| 3 | 对象池增强 | P0 | 阶段2 |
| 4 | 数据池管理 | P0 | 阶段2 |
| 5 | 定时任务调度 | P1 | 无 |
| 6 | 错误处理与日志 | P1 | 无 |
| 7 | 管理 API | P1 | 阶段2-4 |
| 8 | 系统监控 | P2 | 阶段7 |

---

## 阶段 1: 爬虫检测配置化

**目标:** 将硬编码的爬虫检测规则改为 YAML 配置，支持热重载。

**Files:**
- Create: `go-page-server/config/spiders.yaml`
- Modify: `go-page-server/core/spider_detector.go`
- Create: `go-page-server/core/spider_config.go`
- Test: `go-page-server/core/spider_detector_test.go`

### Task 1.1: 创建爬虫配置文件

**Step 1: 创建配置文件**

Create: `go-page-server/config/spiders.yaml`

```yaml
# 爬虫检测配置
# 支持热重载，修改后自动生效

spiders:
  baidu:
    name: "百度蜘蛛"
    patterns:
      - "Baiduspider"
      - "Baiduspider-render"
      - "Baiduspider-image"
      - "Baiduspider-video"
      - "Baiduspider-news"
    enabled: true

  google:
    name: "谷歌蜘蛛"
    patterns:
      - "Googlebot"
      - "Googlebot-Image"
      - "Googlebot-News"
      - "Googlebot-Video"
      - "Mediapartners-Google"
      - "AdsBot-Google"
    enabled: true

  bing:
    name: "必应蜘蛛"
    patterns:
      - "bingbot"
      - "msnbot"
      - "BingPreview"
    enabled: true

  sogou:
    name: "搜狗蜘蛛"
    patterns:
      - "Sogou web spider"
      - "Sogou inst spider"
      - "Sogou spider"
    enabled: true

  360:
    name: "360蜘蛛"
    patterns:
      - "360Spider"
      - "HaosouSpider"
    enabled: true

  bytedance:
    name: "字节蜘蛛"
    patterns:
      - "Bytespider"
      - "ByteDance"
    enabled: true

  yandex:
    name: "Yandex蜘蛛"
    patterns:
      - "YandexBot"
      - "YandexImages"
    enabled: true

  other:
    name: "其他蜘蛛"
    patterns:
      - "Applebot"
      - "DuckDuckBot"
      - "Slurp"
      - "facebookexternalhit"
      - "LinkedInBot"
      - "Twitterbot"
      - "PetalBot"
      - "SemrushBot"
      - "AhrefsBot"
    enabled: true

# 缓存配置
cache:
  enabled: true
  max_size: 10000      # LRU 缓存最大条目数
  ttl_seconds: 3600    # 缓存 TTL (秒)
```

**Step 2: Commit**

```bash
git add go-page-server/config/spiders.yaml
git commit -m "feat: add spider detection yaml config"
```

### Task 1.2: 创建配置加载器

**Step 1: 创建配置加载器文件**

Create: `go-page-server/core/spider_config.go`

```go
package core

import (
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// SpiderConfig 爬虫配置
type SpiderConfig struct {
	Spiders map[string]SpiderRule `yaml:"spiders"`
	Cache   SpiderCacheConfig     `yaml:"cache"`
}

// SpiderRule 单个爬虫规则
type SpiderRule struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
	Enabled  bool     `yaml:"enabled"`
}

// SpiderCacheConfig 缓存配置
type SpiderCacheConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxSize    int  `yaml:"max_size"`
	TTLSeconds int  `yaml:"ttl_seconds"`
}

// SpiderConfigLoader 配置加载器
type SpiderConfigLoader struct {
	configPath string
	config     *SpiderConfig
	mu         sync.RWMutex
	watcher    *fsnotify.Watcher
	onChange   func(*SpiderConfig)
}

// NewSpiderConfigLoader 创建配置加载器
func NewSpiderConfigLoader(configPath string) (*SpiderConfigLoader, error) {
	loader := &SpiderConfigLoader{
		configPath: configPath,
	}

	if err := loader.load(); err != nil {
		return nil, err
	}

	return loader, nil
}

// load 加载配置
func (l *SpiderConfigLoader) load() error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return err
	}

	var config SpiderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	l.mu.Lock()
	l.config = &config
	l.mu.Unlock()

	log.Info().
		Str("path", l.configPath).
		Int("spider_types", len(config.Spiders)).
		Msg("Spider config loaded")

	return nil
}

// GetConfig 获取当前配置
func (l *SpiderConfigLoader) GetConfig() *SpiderConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// WatchChanges 监听配置变化
func (l *SpiderConfigLoader) WatchChanges(onChange func(*SpiderConfig)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	l.watcher = watcher
	l.onChange = onChange

	go func() {
		debounce := time.NewTimer(0)
		<-debounce.C // drain initial

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Debounce: wait 100ms before reload
					debounce.Reset(100 * time.Millisecond)
				}
			case <-debounce.C:
				if err := l.load(); err != nil {
					log.Error().Err(err).Msg("Failed to reload spider config")
				} else {
					log.Info().Msg("Spider config reloaded")
					if l.onChange != nil {
						l.onChange(l.config)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error().Err(err).Msg("Spider config watcher error")
			}
		}
	}()

	return watcher.Add(l.configPath)
}

// Close 关闭监听
func (l *SpiderConfigLoader) Close() error {
	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/spider_config.go
git commit -m "feat: add spider config loader with hot reload"
```

### Task 1.3: 重构爬虫检测器

**Step 1: 修改 spider_detector.go**

Modify: `go-page-server/core/spider_detector.go`

```go
package core

import (
	"regexp"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// SpiderDetector 爬虫检测器
type SpiderDetector struct {
	configLoader *SpiderConfigLoader
	patterns     map[string][]*regexp.Regexp // spider_type -> compiled patterns
	cache        *lru.Cache[string, *cacheEntry]
	mu           sync.RWMutex
}

type cacheEntry struct {
	isSpider   bool
	spiderType string
	expireAt   time.Time
}

// NewSpiderDetector 创建爬虫检测器
func NewSpiderDetector(configLoader *SpiderConfigLoader) (*SpiderDetector, error) {
	config := configLoader.GetConfig()

	// 创建 LRU 缓存
	cacheSize := 10000
	if config.Cache.MaxSize > 0 {
		cacheSize = config.Cache.MaxSize
	}
	cache, err := lru.New[string, *cacheEntry](cacheSize)
	if err != nil {
		return nil, err
	}

	detector := &SpiderDetector{
		configLoader: configLoader,
		patterns:     make(map[string][]*regexp.Regexp),
		cache:        cache,
	}

	// 编译正则表达式
	detector.compilePatterns(config)

	// 监听配置变化
	configLoader.WatchChanges(func(newConfig *SpiderConfig) {
		detector.mu.Lock()
		detector.compilePatterns(newConfig)
		detector.cache.Purge() // 清空缓存
		detector.mu.Unlock()
	})

	return detector, nil
}

// compilePatterns 编译正则表达式
func (d *SpiderDetector) compilePatterns(config *SpiderConfig) {
	d.patterns = make(map[string][]*regexp.Regexp)

	for spiderType, rule := range config.Spiders {
		if !rule.Enabled {
			continue
		}

		var compiled []*regexp.Regexp
		for _, pattern := range rule.Patterns {
			if re, err := regexp.Compile("(?i)" + pattern); err == nil {
				compiled = append(compiled, re)
			}
		}
		d.patterns[spiderType] = compiled
	}
}

// Detect 检测 User-Agent 是否为爬虫
func (d *SpiderDetector) Detect(userAgent string) (bool, string) {
	if userAgent == "" {
		return false, ""
	}

	// 检查缓存
	d.mu.RLock()
	config := d.configLoader.GetConfig()
	d.mu.RUnlock()

	if config.Cache.Enabled {
		if entry, ok := d.cache.Get(userAgent); ok {
			if time.Now().Before(entry.expireAt) {
				return entry.isSpider, entry.spiderType
			}
		}
	}

	// 执行检测
	isSpider, spiderType := d.doDetect(userAgent)

	// 缓存结果
	if config.Cache.Enabled {
		ttl := time.Duration(config.Cache.TTLSeconds) * time.Second
		if ttl == 0 {
			ttl = time.Hour
		}
		d.cache.Add(userAgent, &cacheEntry{
			isSpider:   isSpider,
			spiderType: spiderType,
			expireAt:   time.Now().Add(ttl),
		})
	}

	return isSpider, spiderType
}

// doDetect 执行实际检测
func (d *SpiderDetector) doDetect(userAgent string) (bool, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for spiderType, patterns := range d.patterns {
		for _, re := range patterns {
			if re.MatchString(userAgent) {
				return true, spiderType
			}
		}
	}

	return false, ""
}

// GetStats 获取统计信息
func (d *SpiderDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	patternCounts := make(map[string]int)
	for spiderType, patterns := range d.patterns {
		patternCounts[spiderType] = len(patterns)
	}

	return map[string]interface{}{
		"pattern_counts": patternCounts,
		"cache_size":     d.cache.Len(),
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/spider_detector.go
git commit -m "refactor: spider detector with yaml config and hot reload"
```

### Task 1.4: 添加测试

**Step 1: 创建测试文件**

Create: `go-page-server/core/spider_detector_test.go`

```go
package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpiderDetector(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "spiders.yaml")

	configContent := `
spiders:
  baidu:
    name: "百度蜘蛛"
    patterns:
      - "Baiduspider"
    enabled: true
  google:
    name: "谷歌蜘蛛"
    patterns:
      - "Googlebot"
    enabled: true
  disabled:
    name: "禁用的蜘蛛"
    patterns:
      - "DisabledBot"
    enabled: false

cache:
  enabled: true
  max_size: 100
  ttl_seconds: 60
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 加载配置
	loader, err := NewSpiderConfigLoader(configPath)
	if err != nil {
		t.Fatal(err)
	}

	// 创建检测器
	detector, err := NewSpiderDetector(loader)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		userAgent  string
		wantSpider bool
		wantType   string
	}{
		{
			name:       "百度蜘蛛",
			userAgent:  "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
			wantSpider: true,
			wantType:   "baidu",
		},
		{
			name:       "谷歌蜘蛛",
			userAgent:  "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantSpider: true,
			wantType:   "google",
		},
		{
			name:       "普通浏览器",
			userAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			wantSpider: false,
			wantType:   "",
		},
		{
			name:       "禁用的蜘蛛",
			userAgent:  "DisabledBot/1.0",
			wantSpider: false,
			wantType:   "",
		},
		{
			name:       "空UA",
			userAgent:  "",
			wantSpider: false,
			wantType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSpider, spiderType := detector.Detect(tt.userAgent)
			if isSpider != tt.wantSpider {
				t.Errorf("Detect() isSpider = %v, want %v", isSpider, tt.wantSpider)
			}
			if spiderType != tt.wantType {
				t.Errorf("Detect() spiderType = %v, want %v", spiderType, tt.wantType)
			}
		})
	}
}

func TestSpiderDetectorCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "spiders.yaml")

	configContent := `
spiders:
  baidu:
    patterns: ["Baiduspider"]
    enabled: true
cache:
  enabled: true
  max_size: 100
  ttl_seconds: 60
`
	os.WriteFile(configPath, []byte(configContent), 0644)

	loader, _ := NewSpiderConfigLoader(configPath)
	detector, _ := NewSpiderDetector(loader)

	ua := "Baiduspider/2.0"

	// 第一次调用
	isSpider1, _ := detector.Detect(ua)
	// 第二次调用（应该命中缓存）
	isSpider2, _ := detector.Detect(ua)

	if !isSpider1 || !isSpider2 {
		t.Error("Expected both calls to return true")
	}

	stats := detector.GetStats()
	if stats["cache_size"].(int) != 1 {
		t.Errorf("Expected cache size 1, got %v", stats["cache_size"])
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestSpiderDetector
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/spider_detector_test.go
git commit -m "test: add spider detector tests"
```

---

## 阶段 2: 模板分析器

**目标:** 自动分析模板中各函数的调用次数，用于计算缓存池大小。

**详细计划见:** `docs/plans/2024-01-29-template-analyzer.md`

---

## 阶段 3: 对象池增强

**目标:** 基于模板分析结果自动计算和调整对象池大小。

**详细计划见:** `docs/plans/2024-01-29-object-pool-enhancement.md`

---

## 阶段 4: 数据池管理

**目标:** 实现数据池的加载、刷新、SEO 友好度分析功能。

**详细计划见:** `docs/plans/2024-01-29-data-pool-management.md`

---

## 阶段 5: 定时任务调度

**目标:** 实现基于 cron 的定时任务调度系统。

**详细计划见:** `docs/plans/2024-01-29-scheduler.md`

---

## 阶段 6: 错误处理与日志

**目标:** 统一错误码、响应格式和日志配置。

**详细计划见:** `docs/plans/2024-01-29-error-handling.md`

---

## 阶段 7: 管理 API

**目标:** 实现缓存池管理、模板分析、系统监控等 API。

**详细计划见:** `docs/plans/2024-01-29-admin-api.md`

---

## 阶段 8: 系统监控

**目标:** 实现系统资源监控和告警功能。

**详细计划见:** `docs/plans/2024-01-29-monitoring.md`

---

## 数据库迁移

**目标:** 一键迁移脚本，包含数据库变更和服务切换。

**详细计划见:** `docs/plans/2024-01-29-migration.md`

---

## 执行顺序建议

```
1. 阶段1（爬虫检测）  ─┐
2. 阶段2（模板分析）  ─┼─► 可并行
3. 阶段6（错误处理）  ─┘
         │
         ▼
4. 阶段3（对象池）   ─┐
5. 阶段4（数据池）   ─┼─► 依赖阶段2
         │           ─┘
         ▼
6. 阶段5（定时任务）
         │
         ▼
7. 阶段7（管理API）  ─► 依赖阶段2-4
         │
         ▼
8. 阶段8（系统监控） ─► 依赖阶段7
         │
         ▼
9. 数据库迁移       ─► 最后执行
```
