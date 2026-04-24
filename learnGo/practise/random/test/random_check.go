package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

// --- 配置区域 ---
const (
	TestBaseURL    = "http://localhost:8181" // 你的服务器地址
	TestTotalRuns  = 1000000                 // 总共跑多少次抽奖
	TestWorkerNum  = 20                      // 并发线程数 (加快测试速度)
	TestOperatorID = "TestBot"               // 测试操作员名字
)

// --- 数据结构 (加了 Test 前缀以避免和主程序冲突) ---

type TestListResponse struct {
	Success bool     `json:"success"`
	Names   []string `json:"names"`
}

type TestDrawResponse struct {
	Winners []string `json:"winners"`
	Error   string   `json:"error"`
}

type TestDrawRequest struct {
	Operator string `json:"operator"`
}

// --- 主函数 ---

func main() {
	fmt.Println("========================================")
	fmt.Printf("🔥 开始随机性测试\n")
	fmt.Printf("🎯 目标运行次数: %d 次\n", TestTotalRuns)
	fmt.Printf("🔗 目标服务器: %s\n", TestBaseURL)
	fmt.Println("========================================")

	// 1. 获取名单，计算理论概率
	fmt.Println("⏳ 正在获取当前候选人名单...")
	names, err := getTestNames()
	if err != nil {
		fmt.Printf("❌ 获取名单失败: %v\n", err)
		fmt.Println("💡 提示：请确认你的抽奖后端(cryptoRandPeople.go)是否已启动？端口是否是 8181？")
		return
	}

	if len(names) < 2 {
		fmt.Println("❌ 名单人数少于 2 人，无法进行测试！请先去网页端添加候选人。")
		return
	}

	totalCandidates := len(names)
	// 每次抽2人，总中奖人次 = 运行次数 * 2
	totalWinnersSlots := TestTotalRuns * 2
	expectedCount := float64(totalWinnersSlots) / float64(totalCandidates)

	fmt.Printf("✅ 名单人数: %d 人\n", totalCandidates)
	fmt.Printf("📊 理论预期: 每人应该中奖约 %.0f 次\n\n", expectedCount)

	// 2. 并发执行抽奖
	results := make(chan []string, TestTotalRuns)
	jobs := make(chan int, TestTotalRuns)

	// 启动 worker
	for w := 0; w < TestWorkerNum; w++ {
		go testWorker(jobs, results)
	}

	// 发送任务
	startTime := time.Now()
	// 使用协程发送任务，防止通道阻塞
	go func() {
		for i := 0; i < TestTotalRuns; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	// 3. 收集结果
	stats := make(map[string]int)
	// 初始化 map 确保每个人都有 key
	for _, name := range names {
		stats[name] = 0
	}

	completed := 0
	fmt.Print("🚀 进度: 0%")
	for i := 0; i < TestTotalRuns; i++ {
		winners := <-results
		for _, winner := range winners {
			stats[winner]++
		}
		completed++

		// 每完成 5% 更新一次进度条
		if completed%(TestTotalRuns/20) == 0 {
			fmt.Printf("\r🚀 进度: %.0f%% (%d/%d)", float64(completed)/float64(TestTotalRuns)*100, completed, TestTotalRuns)
		}
	}
	fmt.Println("\n✅ 测试完成！正在分析数据...")
	duration := time.Since(startTime)

	// 4. 打印报告
	printTestReport(stats, names, expectedCount, duration)
}

// --- 辅助函数 ---

// 工作线程：不断请求抽奖接口
func testWorker(jobs <-chan int, results chan<- []string) {
	client := &http.Client{Timeout: 10 * time.Second} // 设置超时
	payload := TestDrawRequest{Operator: TestOperatorID}
	data, _ := json.Marshal(payload)

	for range jobs {
		// 发起 POST 请求
		resp, err := client.Post(TestBaseURL+"/api/draw", "application/json", bytes.NewBuffer(data))
		if err != nil {
			// 如果失败(比如网络错误)，返回空切片
			results <- []string{}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var res TestDrawResponse
		// 忽略 JSON 解析错误，如果解析失败 winners 就是空的
		json.Unmarshal(body, &res)

		if len(res.Winners) > 0 {
			results <- res.Winners
		} else {
			results <- []string{}
		}
	}
}

// 获取名单
func getTestNames() ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(TestBaseURL + "/api/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res TestListResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Names, nil
}

// 打印漂亮的表格
func printTestReport(stats map[string]int, names []string, expected float64, duration time.Duration) {
	fmt.Println("\n=======================================================")
	qps := float64(TestTotalRuns) / duration.Seconds()
	fmt.Printf("⏱️  总耗时: %s | QPS: %.0f req/s\n", duration, qps)
	fmt.Println("=======================================================")

	// 使用 tabwriter 对齐输出
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "姓名\t中奖次数\t偏差值(%)\t评价\t")
	fmt.Fprintln(w, "----\t-------\t--------\t----\t")

	var maxDev float64 = 0

	for _, name := range names {
		count := stats[name]
		diff := float64(count) - expected

		var devPercent float64
		if expected > 0 {
			devPercent = (diff / expected) * 100
		} else {
			devPercent = 0
		}

		// 计算绝对偏差用于统计最大值
		absDev := devPercent
		if absDev < 0 {
			absDev = -absDev
		}
		if absDev > maxDev {
			maxDev = absDev
		}

		evaluation := "✅ 正常"
		if devPercent > 10 || devPercent < -10 {
			evaluation = "⚠️ 偏差较大"
		} else if devPercent > 5 || devPercent < -5 {
			evaluation = "📝 些微波动"
		}

		fmt.Fprintf(w, "%s\t%d\t%+.2f%%\t%s\t\n", name, count, devPercent, evaluation)
	}
	w.Flush()

	fmt.Println("=======================================================")
	fmt.Printf("最大偏差率: %.2f%%\n", maxDev)

	if maxDev < 5.0 {
		fmt.Println("🎉 结论：随机性【极佳】（偏差 < 5%）")
	} else if maxDev < 10.0 {
		fmt.Println("👌 结论：随机性【良好】（偏差 < 10%）")
	} else {
		fmt.Println("❌ 结论：随机性【存疑】，请检查算法逻辑或增加测试次数！")
	}
	fmt.Println("=======================================================")
}
