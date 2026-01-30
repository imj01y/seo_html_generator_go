package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test configuration content
const testSpiderConfig = `
cache:
  enabled: true
  max_size: 100
  ttl_seconds: 60

spiders:
  baidu:
    name: "百度蜘蛛"
    enabled: true
    patterns:
      - "(?i)Baiduspider"
      - "(?i)Baidu-YunGuanCe"
    dns_domains:
      - "baidu.com"

  google:
    name: "谷歌蜘蛛"
    enabled: true
    patterns:
      - "(?i)Googlebot"
    dns_domains:
      - "googlebot.com"

  disabled_spider:
    name: "禁用的蜘蛛"
    enabled: false
    patterns:
      - "(?i)DisabledBot"
    dns_domains: []
`

// createTestConfigFile creates a temporary config file for testing
func createTestConfigFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "spiders.yaml")

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	return configPath
}

func TestSpiderDetector_BasicDetection(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	tests := []struct {
		name       string
		userAgent  string
		wantSpider bool
		wantType   string
		wantName   string
	}{
		{
			name:       "Baidu spider",
			userAgent:  "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
			wantSpider: true,
			wantType:   "baidu",
			wantName:   "百度蜘蛛",
		},
		{
			name:       "Google bot",
			userAgent:  "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantSpider: true,
			wantType:   "google",
			wantName:   "谷歌蜘蛛",
		},
		{
			name:       "Normal browser",
			userAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124 Safari/537.36",
			wantSpider: false,
			wantType:   "",
			wantName:   "",
		},
		{
			name:       "Empty UA",
			userAgent:  "",
			wantSpider: false,
			wantType:   "",
			wantName:   "",
		},
		{
			name:       "Case insensitive - lowercase baiduspider",
			userAgent:  "baiduspider",
			wantSpider: true,
			wantType:   "baidu",
			wantName:   "百度蜘蛛",
		},
		{
			name:       "Case insensitive - uppercase GOOGLEBOT",
			userAgent:  "GOOGLEBOT/2.1",
			wantSpider: true,
			wantType:   "google",
			wantName:   "谷歌蜘蛛",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.userAgent)

			if result.IsSpider != tt.wantSpider {
				t.Errorf("IsSpider = %v, want %v", result.IsSpider, tt.wantSpider)
			}
			if result.SpiderType != tt.wantType {
				t.Errorf("SpiderType = %v, want %v", result.SpiderType, tt.wantType)
			}
			if result.SpiderName != tt.wantName {
				t.Errorf("SpiderName = %v, want %v", result.SpiderName, tt.wantName)
			}
			if result.UserAgent != tt.userAgent {
				t.Errorf("UserAgent = %v, want %v", result.UserAgent, tt.userAgent)
			}
		})
	}
}

func TestSpiderDetector_CacheHit(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	userAgent := "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)"

	// First detection (cache miss)
	result1 := detector.Detect(userAgent)
	stats1 := detector.GetStats()
	cacheMisses1 := stats1["cache_misses"].(int64)
	cacheHits1 := stats1["cache_hits"].(int64)

	if !result1.IsSpider {
		t.Error("First detection should identify spider")
	}
	if cacheMisses1 != 1 {
		t.Errorf("Expected 1 cache miss, got %d", cacheMisses1)
	}
	if cacheHits1 != 0 {
		t.Errorf("Expected 0 cache hits, got %d", cacheHits1)
	}

	// Second detection (cache hit)
	result2 := detector.Detect(userAgent)
	stats2 := detector.GetStats()
	cacheMisses2 := stats2["cache_misses"].(int64)
	cacheHits2 := stats2["cache_hits"].(int64)

	if !result2.IsSpider {
		t.Error("Second detection should identify spider")
	}
	if result2.SpiderType != result1.SpiderType {
		t.Error("Cached result should match original")
	}
	if cacheMisses2 != 1 {
		t.Errorf("Cache misses should still be 1, got %d", cacheMisses2)
	}
	if cacheHits2 != 1 {
		t.Errorf("Expected 1 cache hit, got %d", cacheHits2)
	}

	// Third detection (another cache hit)
	detector.Detect(userAgent)
	stats3 := detector.GetStats()
	cacheHits3 := stats3["cache_hits"].(int64)

	if cacheHits3 != 2 {
		t.Errorf("Expected 2 cache hits, got %d", cacheHits3)
	}
}

func TestSpiderDetector_DisabledRule(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	// Test that disabled spider is not detected
	userAgent := "DisabledBot/1.0"
	result := detector.Detect(userAgent)

	if result.IsSpider {
		t.Error("Disabled spider rule should not match")
	}
}

func TestSpiderDetector_NegativeCache(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	// First detection - not a spider
	result1 := detector.Detect(userAgent)
	if result1.IsSpider {
		t.Error("Should not be detected as spider")
	}

	stats1 := detector.GetStats()
	cacheMisses1 := stats1["cache_misses"].(int64)

	// Second detection - should be cache hit (negative cache)
	result2 := detector.Detect(userAgent)
	if result2.IsSpider {
		t.Error("Should not be detected as spider")
	}

	stats2 := detector.GetStats()
	cacheHits2 := stats2["cache_hits"].(int64)
	cacheMisses2 := stats2["cache_misses"].(int64)

	if cacheHits2 != 1 {
		t.Errorf("Expected 1 cache hit for negative result, got %d", cacheHits2)
	}
	if cacheMisses2 != cacheMisses1 {
		t.Errorf("Cache misses should not increase for cached result")
	}
}

func TestSpiderDetector_CacheTTL(t *testing.T) {
	// Create config with very short TTL for testing
	shortTTLConfig := `
cache:
  enabled: true
  max_size: 100
  ttl_seconds: 1

spiders:
  baidu:
    name: "百度蜘蛛"
    enabled: true
    patterns:
      - "(?i)Baiduspider"
    dns_domains: []
`
	configPath := createTestConfigFile(t, shortTTLConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	userAgent := "Baiduspider/2.0"

	// First detection
	result1 := detector.Detect(userAgent)
	if !result1.IsSpider {
		t.Error("Should detect spider")
	}

	stats1 := detector.GetStats()
	if stats1["cache_misses"].(int64) != 1 {
		t.Error("First detection should be cache miss")
	}

	// Immediate second detection (cache hit)
	detector.Detect(userAgent)
	stats2 := detector.GetStats()
	if stats2["cache_hits"].(int64) != 1 {
		t.Error("Second detection should be cache hit")
	}

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	// Third detection (cache miss due to TTL expiry)
	detector.Detect(userAgent)
	stats3 := detector.GetStats()
	if stats3["cache_misses"].(int64) != 2 {
		t.Errorf("Expected 2 cache misses after TTL expiry, got %d", stats3["cache_misses"].(int64))
	}
}

func TestSpiderDetector_CacheDisabled(t *testing.T) {
	// Create config with cache disabled
	noCacheConfig := `
cache:
  enabled: false
  max_size: 100
  ttl_seconds: 60

spiders:
  baidu:
    name: "百度蜘蛛"
    enabled: true
    patterns:
      - "(?i)Baiduspider"
    dns_domains: []
`
	configPath := createTestConfigFile(t, noCacheConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	userAgent := "Baiduspider/2.0"

	// Multiple detections
	for i := 0; i < 3; i++ {
		result := detector.Detect(userAgent)
		if !result.IsSpider {
			t.Error("Should detect spider")
		}
	}

	stats := detector.GetStats()
	// With cache disabled, cache_size should be 0
	if stats["cache_size"].(int) != 0 {
		t.Errorf("Cache should be empty when disabled, got size %d", stats["cache_size"].(int))
	}
}

func TestSpiderDetector_IsSpider(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	if !detector.IsSpider("Baiduspider/2.0") {
		t.Error("IsSpider should return true for spider UA")
	}

	if detector.IsSpider("Chrome/91.0") {
		t.Error("IsSpider should return false for normal UA")
	}
}

func TestSpiderDetector_GetSpiderInfo(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	info := detector.GetSpiderInfo("baidu")
	if info == nil {
		t.Fatal("GetSpiderInfo should return info for baidu")
	}

	if info.Name != "百度蜘蛛" {
		t.Errorf("Expected name '百度蜘蛛', got '%s'", info.Name)
	}

	if len(info.DNSDomains) == 0 {
		t.Error("DNS domains should not be empty")
	}

	// Test non-existent spider type
	info2 := detector.GetSpiderInfo("nonexistent")
	if info2 != nil {
		t.Error("GetSpiderInfo should return nil for unknown spider type")
	}
}

func TestSpiderDetector_GetAllSpiderTypes(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	types := detector.GetAllSpiderTypes()

	// Should include baidu, google, disabled_spider (even if disabled, it's in config)
	if len(types) < 2 {
		t.Errorf("Expected at least 2 spider types, got %d", len(types))
	}

	// Check that baidu is in the list
	found := false
	for _, spiderType := range types {
		if spiderType == "baidu" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Spider types should include 'baidu'")
	}
}

func TestSpiderDetector_FallbackMode(t *testing.T) {
	// Test fallback mode when config file doesn't exist
	detector, err := NewSpiderDetectorWithConfig("/nonexistent/path/spiders.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file")
	}
	if detector != nil {
		t.Error("Detector should be nil when config loading fails")
	}

	// Test that NewSpiderDetector falls back gracefully
	// This will use fallback since config path doesn't exist
	fallbackDetector := newSpiderDetectorFallback()
	if fallbackDetector == nil {
		t.Fatal("Fallback detector should not be nil")
	}

	// Test that fallback detector works
	result := fallbackDetector.Detect("Baiduspider/2.0")
	if !result.IsSpider {
		t.Error("Fallback detector should detect Baidu spider")
	}
	if result.SpiderType != "baidu" {
		t.Errorf("Expected spider type 'baidu', got '%s'", result.SpiderType)
	}
}

func TestSpiderConfigLoader_Load(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	loader, err := NewSpiderConfigLoader(configPath)
	if err != nil {
		t.Fatalf("Failed to create config loader: %v", err)
	}

	config := loader.GetConfig()
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if !config.Cache.Enabled {
		t.Error("Cache should be enabled")
	}
	if config.Cache.MaxSize != 100 {
		t.Errorf("Expected cache max_size 100, got %d", config.Cache.MaxSize)
	}
	if config.Cache.TTLSeconds != 60 {
		t.Errorf("Expected cache TTL 60, got %d", config.Cache.TTLSeconds)
	}

	rules := loader.GetCompiledRules()
	if len(rules) == 0 {
		t.Error("Should have compiled rules")
	}

	// Check that disabled rules are not in compiled rules
	for _, rule := range rules {
		if rule.Type == "disabled_spider" {
			t.Error("Disabled spider should not be in compiled rules")
		}
	}
}

func TestSpiderConfigLoader_InvalidRegex(t *testing.T) {
	invalidConfig := `
cache:
  enabled: true
  max_size: 100
  ttl_seconds: 60

spiders:
  test:
    name: "Test"
    enabled: true
    patterns:
      - "[invalid(regex"
    dns_domains: []
`
	configPath := createTestConfigFile(t, invalidConfig)

	_, err := NewSpiderConfigLoader(configPath)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestSpiderDetector_MultiplePatterns(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	// Test both Baidu patterns
	tests := []string{
		"Baiduspider/2.0",
		"Baidu-YunGuanCe-Bot",
	}

	for _, ua := range tests {
		result := detector.Detect(ua)
		if !result.IsSpider || result.SpiderType != "baidu" {
			t.Errorf("UA '%s' should be detected as baidu spider", ua)
		}
	}
}

func TestSpiderDetector_ConcurrentAccess(t *testing.T) {
	configPath := createTestConfigFile(t, testSpiderConfig)

	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create spider detector: %v", err)
	}

	// Test concurrent access
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				detector.Detect("Baiduspider/2.0")
				detector.Detect("Chrome/91.0")
				detector.GetStats()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should complete without race conditions
	stats := detector.GetStats()
	if stats["cache_size"].(int) < 1 {
		t.Error("Cache should have entries after concurrent access")
	}
}
