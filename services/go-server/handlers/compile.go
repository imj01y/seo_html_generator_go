// Package handlers contains HTTP request handlers
package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// CompileRequest represents the request body for template compilation
type CompileRequest struct {
	TemplateID   int    `json:"template_id" binding:"required"`
	TemplateName string `json:"template_name"` // Optional: override template name
}

// CompileResponse represents the response for template compilation
type CompileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Stage   string `json:"stage,omitempty"`
}

// CompileHandler handles template compilation requests
type CompileHandler struct {
	handler      *PageHandler
	templatesDir string
}

// NewCompileHandler creates a new compile handler
func NewCompileHandler(handler *PageHandler, templatesDir string) *CompileHandler {
	return &CompileHandler{
		handler:      handler,
		templatesDir: templatesDir,
	}
}

// CompileTemplate handles POST /api/template/compile
func (h *CompileHandler) CompileTemplate(c *gin.Context) {
	startTime := time.Now()

	var req CompileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CompileResponse{
			Success: false,
			Error:   "无效的请求参数: " + err.Error(),
			Stage:   "参数解析",
		})
		return
	}

	log.Info().
		Int("template_id", req.TemplateID).
		Msg("Starting template compilation")

	// 1. Get template from database
	ctx := context.Background()
	template, err := h.getTemplateByID(ctx, req.TemplateID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, CompileResponse{
				Success: false,
				Error:   fmt.Sprintf("模板 ID %d 不存在", req.TemplateID),
				Stage:   "加载模板",
			})
		} else {
			c.JSON(http.StatusInternalServerError, CompileResponse{
				Success: false,
				Error:   "数据库错误: " + err.Error(),
				Stage:   "加载模板",
			})
		}
		return
	}

	templateName := template.Name
	if req.TemplateName != "" {
		templateName = req.TemplateName
	}

	log.Info().
		Str("template_name", templateName).
		Int("content_length", len(template.Content)).
		Msg("Template loaded from database")

	// 2. Validate Jinja2 syntax
	validator := core.NewTemplateValidator()
	result := validator.Validate(template.Content)
	if !result.Valid {
		errMsg := ""
		stage := ""
		for _, e := range result.Errors {
			errMsg = e.Error()
			stage = e.Stage
			break
		}
		c.JSON(http.StatusBadRequest, CompileResponse{
			Success: false,
			Error:   errMsg,
			Stage:   stage,
		})
		return
	}

	log.Info().Msg("Template validation passed")

	// 3. Convert Jinja2 to quicktemplate
	converter := core.NewJinja2ToQuickTemplate(templateName)
	qtplContent, err := converter.Convert(template.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, CompileResponse{
			Success: false,
			Error:   err.Error(),
			Stage:   "语法转换",
		})
		return
	}

	log.Info().
		Int("qtpl_length", len(qtplContent)).
		Msg("Template converted to quicktemplate")

	// 4. Ensure templates directory exists
	if err := os.MkdirAll(h.templatesDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, CompileResponse{
			Success: false,
			Error:   "无法创建模板目录: " + err.Error(),
			Stage:   "保存文件",
		})
		return
	}

	// 5. Save .qtpl file
	qtplPath := filepath.Join(h.templatesDir, templateName+".qtpl")
	if err := os.WriteFile(qtplPath, []byte(qtplContent), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, CompileResponse{
			Success: false,
			Error:   "保存模板文件失败: " + err.Error(),
			Stage:   "保存文件",
		})
		return
	}

	log.Info().
		Str("qtpl_path", qtplPath).
		Msg("Quicktemplate file saved")

	// 6. Run qtc compiler
	cmd := exec.Command("qtc", "-dir="+h.templatesDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, CompileResponse{
			Success: false,
			Error:   string(output),
			Stage:   "quicktemplate 编译",
		})
		return
	}

	log.Info().
		Str("output", string(output)).
		Msg("qtc compilation completed")

	// 7. Run go build
	cmd = exec.Command("go", "build", "-o", "server_new", ".")
	output, err = cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, CompileResponse{
			Success: false,
			Error:   string(output),
			Stage:   "Go 编译",
		})
		return
	}

	log.Info().Msg("Go build completed")

	// 8. Replace binary (on Unix systems)
	if err := os.Rename("server_new", "server"); err != nil {
		// On Windows, may need different handling
		log.Warn().Err(err).Msg("Failed to rename binary, may need manual restart")
	}

	elapsed := time.Since(startTime)
	log.Info().
		Dur("elapsed", elapsed).
		Msg("Template compilation completed successfully")

	// 9. Return success response
	c.JSON(http.StatusOK, CompileResponse{
		Success: true,
		Message: fmt.Sprintf("编译成功，耗时 %v，服务正在重载", elapsed.Round(time.Millisecond)),
	})

	// 10. Trigger graceful restart in background (Unix only)
	// On Windows, the service needs to be restarted manually or via Docker
	go func() {
		time.Sleep(100 * time.Millisecond) // Ensure response is sent
		log.Info().Msg("Compilation complete. Please restart the server to apply changes.")
		// Note: syscall.Kill is not available on Windows
		// In production with Docker on Linux, uncomment the following:
		// syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	}()
}

// getTemplateByID retrieves a template from the database
func (h *CompileHandler) getTemplateByID(ctx context.Context, id int) (*templateData, error) {
	var tmpl templateData

	query := `SELECT id, name, content FROM templates WHERE id = ? AND status = 1 LIMIT 1`
	err := h.handler.db.GetContext(ctx, &tmpl, query, id)
	if err != nil {
		return nil, err
	}

	return &tmpl, nil
}

// templateData represents template data from database
type templateData struct {
	ID      int    `db:"id"`
	Name    string `db:"name"`
	Content string `db:"content"`
}

// PreviewTemplate handles POST /api/template/preview
// This allows previewing the conversion without compiling
func (h *CompileHandler) PreviewTemplate(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// Validate
	validator := core.NewTemplateValidator()
	result := validator.Validate(req.Content)

	if !result.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":  false,
			"errors": result.Errors,
		})
		return
	}

	// Convert
	converter := core.NewJinja2ToQuickTemplate("preview")
	convertResult := converter.ConvertWithValidation(req.Content)

	if len(convertResult.Errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":  false,
			"errors": convertResult.Errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":         true,
		"warnings":      convertResult.Warnings,
		"quicktemplate": convertResult.QuickTemplate,
	})
}

// ValidateTemplate handles POST /api/template/validate
// This validates the template without converting
func (h *CompileHandler) ValidateTemplate(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	validator := core.NewTemplateValidator()
	result := validator.Validate(req.Content)

	errors := make([]map[string]interface{}, 0)
	for _, e := range result.Errors {
		errors = append(errors, map[string]interface{}{
			"stage":      e.Stage,
			"line":       e.Line,
			"message":    e.Message,
			"suggestion": e.Suggestion,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    result.Valid,
		"errors":   errors,
		"warnings": result.Warnings,
	})
}

// CompileStatus handles GET /api/template/compile/status
// Returns the current compilation status
func (h *CompileHandler) CompileStatus(c *gin.Context) {
	// Check if templates directory exists
	templatesExist := false
	if _, err := os.Stat(h.templatesDir); err == nil {
		templatesExist = true
	}

	// Check if qtc is available
	qtcAvailable := false
	if _, err := exec.LookPath("qtc"); err == nil {
		qtcAvailable = true
	}

	// Check if go is available
	goAvailable := false
	if _, err := exec.LookPath("go"); err == nil {
		goAvailable = true
	}

	c.JSON(http.StatusOK, gin.H{
		"templates_dir_exists": templatesExist,
		"templates_dir":        h.templatesDir,
		"qtc_available":        qtcAvailable,
		"go_available":         goAvailable,
		"ready":                templatesExist && qtcAvailable && goAvailable,
	})
}
