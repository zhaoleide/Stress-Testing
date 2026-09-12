package main

import (
	"fmt"
	"log"
	"time"

	"stress-testing/framework"
	"stress-testing/scenarios"
)

func main() {
	// 创建登录压测场景
	scenario := scenarios.NewLoginScenario()

	// 创建压测引擎
	engine := framework.NewEngine(scenario)

	// 获取默认配置并自定义
	config := scenario.Config()

	// 自定义配置参数 - 增加用户数以便生成有意义的图表
	config.UserCount = 500 // 50个用户以便有足够时间监控
	config.Concurrency = 5 // 并发数5，延长执行时间
	config.Params["login_url"] = "https://example.com/apiv2/getVerifyCode"
	config.Params["password"] = "CHANGE_ME"

	// 系统监控默认关闭；启用时请用环境变量注入凭据，勿把真实密码写进代码
	config.MonitorConfig = &framework.MonitorConfig{
		Enabled:  false,
		Host:     "example.com",
		Username: "monitor",
		Password: "", // 或改用 PrivateKey
		Interval: 2 * time.Second,
		Metrics:  []string{"cpu", "memory", "disk", "network"},
	}

	printUsage()

	// 运行压测
	if err := engine.Run(config); err != nil {
		log.Fatalf("压测执行失败: %v", err)
	}

	printNextSteps()
}

func printUsage() {
	fmt.Println("=== 通用压力测试框架 ===")
	fmt.Println("支持可插拔的业务场景和系统监控")
	fmt.Println("当前使用场景: 登录压测场景")
	fmt.Println()
}

func printNextSteps() {
	fmt.Println("\n=== 登录压测完成 ===")
	fmt.Println("1. 查看 reports/ 目录下的详细测试报告")
	fmt.Println("2. 登录压测统计信息已在上方显示")
	fmt.Println("3. 可启用系统监控查看服务器性能")
	fmt.Println("4. 在 scenarios/login_scenario.go 中自定义登录逻辑")
	fmt.Println("\n注意: 纯Go实现，简单高效的登录接口压测工具!")
}
