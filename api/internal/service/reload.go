// Package core provides core functionality for the Go page server
package core

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ReloadManager manages graceful reload of the server
type ReloadManager struct {
	mu           sync.Mutex
	server       *http.Server
	isReloading  bool
	lastReload   time.Time
	reloadCount  int
	execPath     string
	workDir      string
	onBeforeStop func() // Called before stopping the server
}

// NewReloadManager creates a new reload manager
func NewReloadManager(server *http.Server) *ReloadManager {
	execPath, _ := os.Executable()
	workDir, _ := os.Getwd()

	return &ReloadManager{
		server:   server,
		execPath: execPath,
		workDir:  workDir,
	}
}

// SetBeforeStopHandler sets a function to be called before stopping
func (rm *ReloadManager) SetBeforeStopHandler(fn func()) {
	rm.onBeforeStop = fn
}

// TriggerReload initiates a graceful reload
func (rm *ReloadManager) TriggerReload() error {
	rm.mu.Lock()
	if rm.isReloading {
		rm.mu.Unlock()
		log.Warn().Msg("Reload already in progress, skipping")
		return nil
	}
	rm.isReloading = true
	rm.mu.Unlock()

	defer func() {
		rm.mu.Lock()
		rm.isReloading = false
		rm.lastReload = time.Now()
		rm.reloadCount++
		rm.mu.Unlock()
	}()

	log.Info().Msg("Starting graceful reload...")

	// Platform-specific reload
	if runtime.GOOS == "windows" {
		return rm.reloadWindows()
	}
	return rm.reloadUnix()
}

// reloadUnix performs reload on Unix systems using exec
func (rm *ReloadManager) reloadUnix() error {
	log.Info().Msg("Unix reload: Using exec to restart")

	// Call before stop handler
	if rm.onBeforeStop != nil {
		rm.onBeforeStop()
	}

	// Graceful shutdown of current server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := rm.server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	// Execute new process
	cmd := exec.Command(rm.execPath)
	cmd.Dir = rm.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start new process")
		return err
	}

	log.Info().Int("pid", cmd.Process.Pid).Msg("New process started")

	// Exit current process
	os.Exit(0)
	return nil
}

// reloadWindows performs reload on Windows
func (rm *ReloadManager) reloadWindows() error {
	log.Info().Msg("Windows reload: Scheduling restart")

	// On Windows, we create a batch script that will restart the server
	// after the current process exits
	scriptPath := filepath.Join(rm.workDir, "_restart.bat")

	// Write restart script
	scriptContent := "@echo off\n" +
		"timeout /t 2 /nobreak >nul\n" +
		"start \"\" \"" + rm.execPath + "\"\n" +
		"del \"%~f0\"\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create restart script")
		return err
	}

	// Start the restart script
	cmd := exec.Command("cmd", "/c", "start", "/b", scriptPath)
	cmd.Dir = rm.workDir
	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start restart script")
		return err
	}

	// Call before stop handler
	if rm.onBeforeStop != nil {
		rm.onBeforeStop()
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := rm.server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Server stopped, restart script will start new instance")
	os.Exit(0)
	return nil
}

// GetStatus returns the current reload status
func (rm *ReloadManager) GetStatus() map[string]interface{} {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	return map[string]interface{}{
		"is_reloading": rm.isReloading,
		"last_reload":  rm.lastReload,
		"reload_count": rm.reloadCount,
		"platform":     runtime.GOOS,
		"exec_path":    rm.execPath,
	}
}

// CompileAndReload compiles the template and triggers a reload
func CompileAndReload(templatesDir, templateName string, rm *ReloadManager) error {
	log.Info().Str("template", templateName).Msg("Starting compile and reload")

	// Step 1: Run qtc
	cmd := exec.Command("qtc", "-dir="+templatesDir)
	cmd.Dir = filepath.Dir(templatesDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("output", string(output)).Msg("qtc compilation failed")
		return err
	}
	log.Info().Msg("qtc compilation successful")

	// Step 2: Run go build
	execPath, _ := os.Executable()
	newExecPath := execPath + ".new"
	if runtime.GOOS == "windows" {
		newExecPath = execPath[:len(execPath)-4] + "_new.exe"
	}

	cmd = exec.Command("go", "build", "-o", newExecPath, ".")
	cmd.Dir = filepath.Dir(templatesDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("output", string(output)).Msg("go build failed")
		return err
	}
	log.Info().Str("output", newExecPath).Msg("go build successful")

	// Step 3: Replace executable (platform specific)
	if runtime.GOOS != "windows" {
		// On Unix, we can rename while running
		if err := os.Rename(newExecPath, execPath); err != nil {
			log.Error().Err(err).Msg("Failed to replace executable")
			return err
		}
	}
	// On Windows, the new exe will be used on next restart

	// Step 4: Trigger reload if manager is provided
	if rm != nil {
		return rm.TriggerReload()
	}

	log.Info().Msg("Compilation complete. Manual restart required.")
	return nil
}
