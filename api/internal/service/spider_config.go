// Package core contains the core business logic
package core

import (
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// SpiderCacheConfig holds cache configuration for spider detection
type SpiderCacheConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxSize    int  `yaml:"max_size"`
	TTLSeconds int  `yaml:"ttl_seconds"`
}

// SpiderRule defines a single spider detection rule
type SpiderRule struct {
	Name       string   `yaml:"name"`
	Enabled    bool     `yaml:"enabled"`
	Patterns   []string `yaml:"patterns"`
	DNSDomains []string `yaml:"dns_domains"`
}

// SpiderConfig holds the complete spider configuration
type SpiderConfig struct {
	Cache   SpiderCacheConfig     `yaml:"cache"`
	Spiders map[string]SpiderRule `yaml:"spiders"`
}

// CompiledSpiderRule contains a spider rule with compiled regex patterns
type CompiledSpiderRule struct {
	Type       string
	Name       string
	Patterns   []*regexp.Regexp
	DNSDomains []string
	Enabled    bool
}

// SpiderConfigLoader handles loading and hot-reloading of spider configuration
type SpiderConfigLoader struct {
	configPath    string
	config        *SpiderConfig
	compiledRules []*CompiledSpiderRule
	rulesByType   map[string]*CompiledSpiderRule
	mu            sync.RWMutex
	watcher       *fsnotify.Watcher
	onChange      func(*SpiderConfig, []*CompiledSpiderRule)
	stopChan      chan struct{}
	debounceTimer *time.Timer
	debounceMu    sync.Mutex
}

// NewSpiderConfigLoader creates a new configuration loader
func NewSpiderConfigLoader(configPath string) (*SpiderConfigLoader, error) {
	loader := &SpiderConfigLoader{
		configPath:  configPath,
		rulesByType: make(map[string]*CompiledSpiderRule),
		stopChan:    make(chan struct{}),
	}

	// Initial load
	if err := loader.Load(); err != nil {
		return nil, err
	}

	return loader, nil
}

// Load reads and parses the configuration file
func (l *SpiderConfigLoader) Load() error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return err
	}

	var config SpiderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	// Compile regex patterns
	compiledRules, rulesByType, err := l.compileRules(&config)
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.config = &config
	l.compiledRules = compiledRules
	l.rulesByType = rulesByType
	l.mu.Unlock()

	log.Info().
		Str("path", l.configPath).
		Int("spider_count", len(config.Spiders)).
		Bool("cache_enabled", config.Cache.Enabled).
		Int("cache_max_size", config.Cache.MaxSize).
		Int("cache_ttl", config.Cache.TTLSeconds).
		Msg("Spider configuration loaded")

	return nil
}

// compileRules compiles all regex patterns in the configuration
func (l *SpiderConfigLoader) compileRules(config *SpiderConfig) ([]*CompiledSpiderRule, map[string]*CompiledSpiderRule, error) {
	var rules []*CompiledSpiderRule
	rulesByType := make(map[string]*CompiledSpiderRule)

	for spiderType, rule := range config.Spiders {
		if !rule.Enabled {
			log.Debug().Str("spider", spiderType).Msg("Spider rule disabled, skipping")
			continue
		}

		compiled := &CompiledSpiderRule{
			Type:       spiderType,
			Name:       rule.Name,
			DNSDomains: rule.DNSDomains,
			Enabled:    rule.Enabled,
		}

		for _, pattern := range rule.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.Error().
					Err(err).
					Str("spider", spiderType).
					Str("pattern", pattern).
					Msg("Failed to compile regex pattern")
				return nil, nil, err
			}
			compiled.Patterns = append(compiled.Patterns, re)
		}

		rules = append(rules, compiled)
		rulesByType[spiderType] = compiled
	}

	return rules, rulesByType, nil
}

// GetConfig returns the current configuration (thread-safe)
func (l *SpiderConfigLoader) GetConfig() *SpiderConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// GetCompiledRules returns the compiled spider rules (thread-safe)
func (l *SpiderConfigLoader) GetCompiledRules() []*CompiledSpiderRule {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.compiledRules
}

// GetRuleByType returns a specific spider rule by type (thread-safe)
func (l *SpiderConfigLoader) GetRuleByType(spiderType string) *CompiledSpiderRule {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rulesByType[spiderType]
}

// OnChange sets a callback function to be called when configuration changes
func (l *SpiderConfigLoader) OnChange(callback func(*SpiderConfig, []*CompiledSpiderRule)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onChange = callback
}

// WatchChanges starts watching the configuration file for changes
func (l *SpiderConfigLoader) WatchChanges() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	l.watcher = watcher

	go l.watchLoop()

	if err := watcher.Add(l.configPath); err != nil {
		watcher.Close()
		return err
	}

	log.Info().Str("path", l.configPath).Msg("Started watching spider configuration file")
	return nil
}

// watchLoop handles file system events
func (l *SpiderConfigLoader) watchLoop() {
	for {
		select {
		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			// Handle write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				l.debounceReload()
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("File watcher error")

		case <-l.stopChan:
			return
		}
	}
}

// debounceReload reloads configuration with debouncing (100ms)
func (l *SpiderConfigLoader) debounceReload() {
	l.debounceMu.Lock()
	defer l.debounceMu.Unlock()

	if l.debounceTimer != nil {
		l.debounceTimer.Stop()
	}

	l.debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
		log.Info().Msg("Configuration file changed, reloading...")

		if err := l.Load(); err != nil {
			log.Error().Err(err).Msg("Failed to reload spider configuration")
			return
		}

		// Call onChange callback if set
		l.mu.RLock()
		callback := l.onChange
		config := l.config
		rules := l.compiledRules
		l.mu.RUnlock()

		if callback != nil {
			callback(config, rules)
		}

		log.Info().Msg("Spider configuration reloaded successfully")
	})
}

// Stop stops watching for configuration changes
func (l *SpiderConfigLoader) Stop() {
	close(l.stopChan)

	l.debounceMu.Lock()
	if l.debounceTimer != nil {
		l.debounceTimer.Stop()
	}
	l.debounceMu.Unlock()

	if l.watcher != nil {
		l.watcher.Close()
	}

	log.Info().Msg("Spider configuration watcher stopped")
}

// DefaultSpiderConfigPath returns the default path for spider configuration
func DefaultSpiderConfigPath() string {
	return "config/spiders.yaml"
}
