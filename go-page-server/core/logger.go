// Package core provides logging configuration and middleware
package core

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds the logging configuration
type LogConfig struct {
	// Level is the minimum log level (debug, info, warn, error, fatal, panic)
	Level string `yaml:"level" json:"level"`
	// Format is the log format (json or console)
	Format string `yaml:"format" json:"format"`
	// Output is the output destination (stdout, file, or both)
	Output string `yaml:"output" json:"output"`
	// FilePath is the log file path (required when output is file or both)
	FilePath string `yaml:"file_path" json:"file_path"`
	// MaxSize is the maximum size in megabytes before rotation
	MaxSize int `yaml:"max_size" json:"max_size"`
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int `yaml:"max_backups" json:"max_backups"`
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int `yaml:"max_age" json:"max_age"`
	// Compress determines if rotated files should be compressed
	Compress bool `yaml:"compress" json:"compress"`
}

// DefaultLogConfig returns a LogConfig with sensible defaults
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/app.log",
		MaxSize:    100, // 100 MB
		MaxBackups: 5,
		MaxAge:     30, // 30 days
		Compress:   true,
	}
}

// SetupLogger configures the global zerolog logger based on LogConfig
func SetupLogger(cfg *LogConfig) error {
	if cfg == nil {
		cfg = DefaultLogConfig()
	}

	// Parse log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339

	// Build writers based on output configuration
	var writers []io.Writer

	switch cfg.Output {
	case "stdout":
		writers = append(writers, buildStdoutWriter(cfg.Format))
	case "file":
		fileWriter, err := buildFileWriter(cfg)
		if err != nil {
			return err
		}
		writers = append(writers, fileWriter)
	case "both":
		writers = append(writers, buildStdoutWriter(cfg.Format))
		fileWriter, err := buildFileWriter(cfg)
		if err != nil {
			return err
		}
		writers = append(writers, fileWriter)
	default:
		writers = append(writers, buildStdoutWriter(cfg.Format))
	}

	// Create multi writer
	multiWriter := io.MultiWriter(writers...)

	// Set global logger
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Caller().Logger()

	log.Info().
		Str("level", cfg.Level).
		Str("format", cfg.Format).
		Str("output", cfg.Output).
		Msg("Logger initialized")

	return nil
}

// buildStdoutWriter creates a writer for stdout based on format
func buildStdoutWriter(format string) io.Writer {
	if format == "console" {
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
	}
	return os.Stdout
}

// buildFileWriter creates a lumberjack rotated file writer
func buildFileWriter(cfg *LogConfig) (io.Writer, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}, nil
}

// RequestLogger returns a gin middleware for request logging
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID
		requestID := generateRequestID()
		c.Set("request_id", requestID)

		// Record start time
		startTime := time.Now()

		// Get request path
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)

		// Get status code
		statusCode := c.Writer.Status()

		// Get client IP
		clientIP := c.ClientIP()

		// Get response size
		responseSize := c.Writer.Size()

		// Build log event
		event := log.Info()
		if statusCode >= 500 {
			event = log.Error()
		} else if statusCode >= 400 {
			event = log.Warn()
		}

		event.
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("client_ip", clientIP).
			Int("response_size", responseSize).
			Str("user_agent", c.Request.UserAgent()).
			Msg("HTTP request")

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				log.Error().
					Str("request_id", requestID).
					Err(e.Err).
					Int("type", int(e.Type)).
					Msg("Request error")
			}
		}
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Use timestamp + random suffix for simplicity
	// Format: YYYYMMDDHHMMSS-XXXXXX
	now := time.Now()
	return now.Format("20060102150405") + "-" + randomString(6)
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond) // Ensure different values
	}
	return string(b)
}

// GetLogger returns a logger with the given component name
func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// WithRequestID returns a logger with request ID from gin context
func WithRequestID(c *gin.Context) zerolog.Logger {
	requestID := ""
	if id, exists := c.Get("request_id"); exists {
		if idStr, ok := id.(string); ok {
			requestID = idStr
		}
	}
	return log.With().Str("request_id", requestID).Logger()
}
