package controller

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"vigo/bootstrap"
	"vigo/framework/db"
	"vigo/framework/mvc"
)

// MigrationController 数据迁移控制器
type MigrationController struct {
	BaseController
}

// Index 迁移管理页面
func (c *MigrationController) Index(ctx *mvc.Context) {
	ctx.HTML(200, "migration/index.html", map[string]interface{}{
		"title": "数据迁移管理",
	})
}

// Status 查看迁移状态
// @Summary 查看迁移状态
// @Tags 数据迁移
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/migration/status [get]
func (c *MigrationController) Status(ctx *mvc.Context) {
	// 检查数据库连接
	if db.GlobalDB == nil {
		ctx.Error(500, "数据库未连接，请检查数据库配置")
		return
	}

	// 检查 migrations 目录是否存在
	if _, err := os.Stat("database/migrations"); os.IsNotExist(err) {
		// 目录不存在，返回空状态而不是错误
		ctx.Success(map[string]interface{}{
			"current_version": "0",
			"total":           0,
			"applied":         []map[string]interface{}{},
			"pending":         []map[string]interface{}{},
			"message":         "migrations 目录不存在，暂无迁移记录",
		})
		return
	}

	migrator := db.NewMigrator(db.GlobalDB, "migrations")

	// 注册迁移
	bootstrap.RegisterMigrations(migrator)

	// 从目录加载迁移（验证文件存在性）
	err := migrator.LoadMigrationsFromDir("database/migrations")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("加载迁移失败：%v", err))
		return
	}

	// 获取状态
	applied, pending, err := migrator.Status()
	if err != nil {
		ctx.Error(500, fmt.Sprintf("获取迁移状态失败：%v", err))
		return
	}

	// 获取当前版本
	currentVersion, _ := migrator.GetCurrentVersion()

	ctx.Success(map[string]interface{}{
		"current_version": currentVersion,
		"total":           len(applied) + len(pending),
		"applied":         formatMigrations(applied),
		"pending":         formatMigrations(pending),
	})
}

// Migrate 执行迁移
// @Summary 执行数据迁移
// @Tags 数据迁移
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/migration/migrate [post]
func (c *MigrationController) Migrate(ctx *mvc.Context) {
	migrator := db.NewMigrator(db.GlobalDB, "migrations")

	// 注册迁移
	bootstrap.RegisterMigrations(migrator)

	// 从目录加载迁移（验证文件存在性）
	err := migrator.LoadMigrationsFromDir("database/migrations")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("加载迁移失败：%v", err))
		return
	}

	// 执行迁移
	err = migrator.Migrate()
	if err != nil {
		ctx.Error(500, fmt.Sprintf("执行迁移失败：%v", err))
		return
	}

	ctx.Success(map[string]interface{}{
		"message": "迁移执行成功",
	})
}

// Rollback 回滚迁移
// @Summary 回滚数据迁移
// @Tags 数据迁移
// @Param steps query int false "回滚步数" default(1)
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/migration/rollback [post]
func (c *MigrationController) Rollback(ctx *mvc.Context) {
	steps := ctx.Input("steps")
	if steps == "" {
		steps = "1"
	}

	var stepCount int
	fmt.Sscanf(steps, "%d", &stepCount)
	if stepCount <= 0 {
		stepCount = 1
	}

	migrator := db.NewMigrator(db.GlobalDB, "migrations")

	// 从目录加载迁移
	err := migrator.LoadMigrationsFromDir("database/migrations")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("加载迁移失败：%v", err))
		return
	}

	// 回滚迁移
	err = migrator.Rollback(stepCount)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("回滚迁移失败：%v", err))
		return
	}

	ctx.Success(map[string]interface{}{
		"message": fmt.Sprintf("成功回滚 %d 步迁移", stepCount),
	})
}

// Reset 重置所有迁移
// @Summary 重置所有数据迁移（危险操作）
// @Tags 数据迁移
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/migration/reset [post]
func (c *MigrationController) Reset(ctx *mvc.Context) {
	migrator := db.NewMigrator(db.GlobalDB, "migrations")

	// 从目录加载迁移
	err := migrator.LoadMigrationsFromDir("database/migrations")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("加载迁移失败：%v", err))
		return
	}

	// 重置迁移
	err = migrator.Reset()
	if err != nil {
		ctx.Error(500, fmt.Sprintf("重置迁移失败：%v", err))
		return
	}

	ctx.Success(map[string]interface{}{
		"message": "所有迁移已重置",
	})
}

// formatMigrations 格式化迁移列表
func formatMigrations(migrations []*db.Migration) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(migrations))
	for _, m := range migrations {
		result = append(result, map[string]interface{}{
			"version":    m.Version,
			"name":       m.Name,
			"applied_at": m.AppliedAt,
		})
	}
	return result
}

// RunMigration 命令行运行迁移（供 CLI 使用）
func RunMigration(action string, args ...string) error {
	migrator := db.NewMigrator(db.GlobalDB, "migrations")

	// 从目录加载迁移
	err := migrator.LoadMigrationsFromDir("database/migrations")
	if err != nil {
		return fmt.Errorf("加载迁移失败：%v", err)
	}

	switch action {
	case "migrate":
		log.Println("Running migrations...")
		return migrator.Migrate()

	case "rollback":
		steps := 1
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &steps)
		}
		log.Printf("Rolling back %d migration(s)...", steps)
		return migrator.Rollback(steps)

	case "reset":
		log.Println("Resetting all migrations...")
		return migrator.Reset()

	case "status":
		applied, pending, err := migrator.Status()
		if err != nil {
			return err
		}

		currentVersion, _ := migrator.GetCurrentVersion()
		fmt.Printf("Current version: %d\n", currentVersion)
		fmt.Printf("Applied migrations: %d\n", len(applied))
		fmt.Printf("Pending migrations: %d\n", len(pending))

		if len(pending) > 0 {
			fmt.Println("\nPending migrations:")
			for _, m := range pending {
				fmt.Printf("  - %d: %s\n", m.Version, m.Name)
			}
		}

		return nil

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// Upload 上传迁移文件
// @Summary 上传迁移文件
// @Tags 数据迁移
// @Accept multipart/form-data
// @Param files formData file true "迁移文件"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/migration/upload [post]
func (c *MigrationController) Upload(ctx *mvc.Context) {
	// 确保上传目录存在
	uploadDir := "database/migrations/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		ctx.Error(500, fmt.Sprintf("创建上传目录失败：%v", err))
		return
	}

	// 获取上传的文件
	files := ctx.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		ctx.Error(400, "请选择要上传的文件")
		return
	}

	uploadedCount := 0
	for _, fileHeader := range files {
		// 检查文件类型
		ext := filepath.Ext(fileHeader.Filename)
		if ext != ".sql" && ext != ".go" {
			ctx.Error(400, fmt.Sprintf("不支持的文件格式：%s，仅支持 .sql 或 .go", ext))
			return
		}

		// 打开上传的文件
		srcFile, err := fileHeader.Open()
		if err != nil {
			ctx.Error(500, fmt.Sprintf("打开文件失败：%v", err))
			return
		}
		defer srcFile.Close()

		// 创建目标文件
		destPath := filepath.Join(uploadDir, fileHeader.Filename)
		destFile, err := os.Create(destPath)
		if err != nil {
			ctx.Error(500, fmt.Sprintf("创建文件失败：%v", err))
			return
		}
		defer destFile.Close()

		// 复制文件内容
		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			ctx.Error(500, fmt.Sprintf("保存文件失败：%v", err))
			return
		}

		uploadedCount++
		log.Printf("迁移文件上传成功：%s -> %s", fileHeader.Filename, destPath)
	}

	// 将上传的文件移动到 migrations 目录
	if err := moveUploadedFiles(uploadDir, "database/migrations"); err != nil {
		ctx.Error(500, fmt.Sprintf("移动文件失败：%v", err))
		return
	}

	ctx.Success(map[string]interface{}{
		"message": fmt.Sprintf("成功上传 %d 个迁移文件", uploadedCount),
		"count":   uploadedCount,
	})
}

// moveUploadedFiles 将上传的文件移动到 migrations 目录
func moveUploadedFiles(uploadDir, targetDir string) error {
	files, err := filepath.Glob(filepath.Join(uploadDir, "*"))
	if err != nil {
		return err
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		targetPath := filepath.Join(targetDir, fileName)

		// 如果目标文件已存在，跳过
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		// 移动文件
		if err := os.Rename(file, targetPath); err != nil {
			return err
		}
	}

	return nil
}
