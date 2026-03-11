package main

import (
	"fmt"
	"os"

	"vigo/framework/cli"
)

// 数据迁移命令行工具
// 使用方式:
//   go run cmd/migrate/main.go migrate
//   go run cmd/migrate/main.go rollback
//   go run cmd/migrate/main.go rollback 3
//   go run cmd/migrate/main.go reset
//   go run cmd/migrate/main.go status
//   go run cmd/migrate/main.go create create_users_table

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "migrate":
		migrateCmd := &cli.MigrateCommand{}
		migrateCmd.Run(append([]string{"migrate"}, args...))
		
	case "rollback":
		migrateCmd := &cli.MigrateCommand{}
		migrateCmd.Run(append([]string{"rollback"}, args...))
		
	case "reset":
		migrateCmd := &cli.MigrateCommand{}
		migrateCmd.Run(append([]string{"reset"}, args...))
		
	case "status":
		migrateCmd := &cli.MigrateCommand{}
		migrateCmd.Run(append([]string{"status"}, args...))
		
	case "create":
		migrateCmd := &cli.MigrateCommand{}
		migrateCmd.Run(append([]string{"create"}, args...))
		
	default:
		fmt.Printf("未知命令：%s\n", command)
		showHelp()
	}
}

func showHelp() {
	fmt.Println("Vigo Framework Migration Tool")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  go run cmd/migrate/main.go <command> [options]")
	fmt.Println("")
	fmt.Println("可用命令:")
	fmt.Println("  migrate              执行所有未应用的迁移")
	fmt.Println("  rollback [steps]     回滚迁移（默认 1 步）")
	fmt.Println("  reset                重置所有迁移")
	fmt.Println("  status               查看迁移状态")
	fmt.Println("  create <name>        创建新的迁移文件")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  go run cmd/migrate/main.go migrate")
	fmt.Println("  go run cmd/migrate/main.go rollback")
	fmt.Println("  go run cmd/migrate/main.go rollback 3")
	fmt.Println("  go run cmd/migrate/main.go reset")
	fmt.Println("  go run cmd/migrate/main.go status")
	fmt.Println("  go run cmd/migrate/main.go create create_users_table")
}
