package main

import (
	"fmt"
	"os"

	"vigo/framework/generator"
	"vigo/framework/idl"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "vigoctl",
		Usage: "Vigo 框架代码生成工具",
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "生成代码",
				Subcommands: []*cli.Command{
					{
						Name:  "api",
						Usage: "从 IDL 生成 API 代码",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "input",
								Aliases:  []string{"i"},
								Usage:    "IDL 文件路径",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "output",
								Aliases:  []string{"o"},
								Usage:    "输出目录",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "style",
								Value: "mvc",
								Usage: "代码风格 (mvc, rpc)",
							},
						},
						Action: func(c *cli.Context) error {
							return generator.GenerateAPI(c.String("input"), c.String("output"), c.String("style"))
						},
					},
					{
						Name:  "model",
						Usage: "从数据库表生成模型",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "table",
								Aliases:  []string{"t"},
								Usage:    "表名",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "output",
								Aliases:  []string{"o"},
								Usage:    "输出目录",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "dsn",
								Usage: "数据库连接字符串",
							},
						},
						Action: func(c *cli.Context) error {
							return generator.GenerateModel(c.String("table"), c.String("output"), c.String("dsn"))
						},
					},
					{
						Name:  "service",
						Usage: "生成微服务模板",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Aliases:  []string{"n"},
								Usage:    "服务名称",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "output",
								Aliases:  []string{"o"},
								Usage:    "输出目录",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "type",
								Value: "http",
								Usage: "服务类型 (http, grpc)",
							},
						},
						Action: func(c *cli.Context) error {
							return generator.GenerateService(c.String("name"), c.String("output"), c.String("type"))
						},
					},
					{
						Name:  "middleware",
						Usage: "生成中间件模板",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Aliases:  []string{"n"},
								Usage:    "中间件名称",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "output",
								Aliases:  []string{"o"},
								Usage:    "输出目录",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							return generator.GenerateMiddleware(c.String("name"), c.String("output"))
						},
					},
				},
			},
			{
				Name:  "idl",
				Usage: "IDL 相关工具",
				Subcommands: []*cli.Command{
					{
						Name:  "init",
						Usage: "初始化 IDL 项目",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "dir",
								Aliases: []string{"d"},
								Value:   "./idl",
								Usage:   "输出目录",
							},
						},
						Action: func(c *cli.Context) error {
							return idl.InitProject(c.String("dir"))
						},
					},
					{
						Name:  "validate",
						Usage: "验证 IDL 文件",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "input",
								Aliases:  []string{"i"},
								Usage:    "IDL 文件路径",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							return idl.Validate(c.String("input"))
						},
					},
				},
			},
			{
				Name:  "upgrade",
				Usage: "升级框架版本",
				Action: func(c *cli.Context) error {
					return generator.Upgrade()
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}
}
