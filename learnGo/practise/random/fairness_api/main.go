package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type ListResponse struct {
	Success bool     `json:"success"`
	Msg     string   `json:"msg"`
	Names   []string `json:"names"`
}

type DrawRequest struct {
	Operator string `json:"operator"`
	Count    int    `json:"count"`
}

type DrawResponse struct {
	Winners []string `json:"winners"`
	Error   string   `json:"error"`
}

type Config struct {
	BaseURL       string
	Draws         int
	WeeklyDraws   int
	Count         int
	Timeout       time.Duration
	ProgressEvery int
	Sleep         time.Duration
	Operator      string
	Yes           bool
}

type PersonStat struct {
	Name         string
	Wins         int
	VirtualWeeks int
	MaxWeekWins  int
}

func main() {
	cfg := Config{}
	timeoutSeconds := 10
	sleepMs := 0

	flag.StringVar(&cfg.BaseURL, "base-url", "http://localhost:8181", "lottery service base URL")
	flag.IntVar(&cfg.Draws, "draws", 100000, "number of /api/draw calls")
	flag.IntVar(&cfg.WeeklyDraws, "weekly-draws", 7, "draw calls per virtual week in this report")
	flag.IntVar(&cfg.Count, "count", 1, "winner count per /api/draw call")
	flag.IntVar(&timeoutSeconds, "timeout", 10, "HTTP timeout in seconds")
	flag.IntVar(&cfg.ProgressEvery, "progress-every", 1000, "print progress every N draw calls")
	flag.IntVar(&sleepMs, "sleep-ms", 0, "sleep milliseconds between draw calls")
	flag.StringVar(&cfg.Operator, "operator", "fairness-api-test", "operator name sent to /api/draw")
	flag.BoolVar(&cfg.Yes, "yes", false, "skip confirmation prompt")
	flag.Parse()

	cfg.Timeout = time.Duration(timeoutSeconds) * time.Second
	cfg.Sleep = time.Duration(sleepMs) * time.Millisecond

	if err := validateConfig(cfg); err != nil {
		exitf("配置错误: %v", err)
	}
	if !cfg.Yes {
		confirmOrExit(cfg)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	names, err := fetchNames(client, cfg.BaseURL)
	if err != nil {
		exitf("获取名单失败: %v", err)
	}
	if len(names) < cfg.Count {
		exitf("名单人数 %d 小于每次抽奖人数 %d", len(names), cfg.Count)
	}

	stats, weeklyTotal, err := runDraws(client, names, cfg)
	if err != nil {
		exitf("测试失败: %v", err)
	}

	printReport(names, stats, weeklyTotal, cfg)
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("base-url 不能为空")
	}
	if cfg.Draws <= 0 {
		return fmt.Errorf("draws 必须大于 0")
	}
	if cfg.WeeklyDraws <= 0 {
		return fmt.Errorf("weekly-draws 必须大于 0")
	}
	if cfg.Count <= 0 {
		return fmt.Errorf("count 必须大于 0")
	}
	if strings.TrimSpace(cfg.Operator) == "" {
		return fmt.Errorf("operator 不能为空")
	}
	return nil
}

func confirmOrExit(cfg Config) {
	fmt.Println("注意：这个测试会调用真实 /api/draw，并写入你的真实 history.jsonl。")
	fmt.Println("另外，weekly-draws 只用于本测试报告分组；服务端是否重置权重仍取决于服务端当前时间和历史。")
	fmt.Printf("即将调用 %s/api/draw 共 %d 次，每次 count=%d。输入 RUN 继续: ",
		strings.TrimRight(cfg.BaseURL, "/"), cfg.Draws, cfg.Count)

	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if strings.TrimSpace(text) != "RUN" {
		fmt.Println("已取消。")
		os.Exit(0)
	}
}

func fetchNames(client *http.Client, baseURL string) ([]string, error) {
	url := strings.TrimRight(baseURL, "/") + "/api/list"
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result ListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("%s", result.Msg)
	}

	names := make([]string, 0, len(result.Names))
	seen := make(map[string]bool, len(result.Names))
	for _, name := range result.Names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func runDraws(client *http.Client, names []string, cfg Config) (map[string]*PersonStat, []map[string]int, error) {
	stats := make(map[string]*PersonStat, len(names))
	for _, name := range names {
		stats[name] = &PersonStat{Name: name}
	}

	weeklyTotal := make([]map[string]int, 0, cfg.Draws/cfg.WeeklyDraws+1)
	currentWeek := make(map[string]int, len(names))

	start := time.Now()
	for i := 0; i < cfg.Draws; i++ {
		if i > 0 && i%cfg.WeeklyDraws == 0 {
			commitWeek(stats, currentWeek)
			weeklyTotal = append(weeklyTotal, currentWeek)
			currentWeek = make(map[string]int, len(names))
		}

		winners, err := drawOnce(client, cfg, i+1)
		if err != nil {
			return nil, nil, fmt.Errorf("第 %d 次抽奖失败: %w", i+1, err)
		}

		for _, winner := range winners {
			stat, ok := stats[winner]
			if !ok {
				stat = &PersonStat{Name: winner}
				stats[winner] = stat
			}
			stat.Wins++
			currentWeek[winner]++
		}

		if cfg.ProgressEvery > 0 && (i+1)%cfg.ProgressEvery == 0 {
			fmt.Printf("progress: %d/%d draw calls, elapsed=%s\n", i+1, cfg.Draws, time.Since(start).Round(time.Second))
		}
		if cfg.Sleep > 0 {
			time.Sleep(cfg.Sleep)
		}
	}

	commitWeek(stats, currentWeek)
	weeklyTotal = append(weeklyTotal, currentWeek)
	return stats, weeklyTotal, nil
}

func drawOnce(client *http.Client, cfg Config, seq int) ([]string, error) {
	reqBody := DrawRequest{
		Operator: fmt.Sprintf("%s-%d", cfg.Operator, seq),
		Count:    cfg.Count,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/api/draw"
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result DrawResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	if len(result.Winners) == 0 {
		return nil, fmt.Errorf("接口返回 winners 为空")
	}
	return result.Winners, nil
}

func commitWeek(stats map[string]*PersonStat, week map[string]int) {
	for name, wins := range week {
		stat, ok := stats[name]
		if !ok {
			stat = &PersonStat{Name: name}
			stats[name] = stat
		}
		if wins > 0 {
			stat.VirtualWeeks++
		}
		if wins > stat.MaxWeekWins {
			stat.MaxWeekWins = wins
		}
	}
}

func printReport(names []string, stats map[string]*PersonStat, weeklyTotal []map[string]int, cfg Config) {
	totalWinnerSlots := 0
	for _, stat := range stats {
		totalWinnerSlots += stat.Wins
	}

	expected := float64(totalWinnerSlots) / float64(len(names))
	minWins := math.MaxInt
	maxWins := 0
	chiSquare := 0.0

	fmt.Println()
	fmt.Println("=== API Weighted Lottery Fairness Report ===")
	fmt.Printf("base_url=%s\n", cfg.BaseURL)
	fmt.Printf("people=%d draw_calls=%d count=%d winner_slots=%d\n", len(names), cfg.Draws, cfg.Count, totalWinnerSlots)
	fmt.Printf("virtual_weekly_draw_calls=%d virtual_weeks=%d\n", cfg.WeeklyDraws, len(weeklyTotal))
	fmt.Println()
	fmt.Printf("%-16s %-10s %-12s %-10s %-14s %-14s\n",
		"name", "wins", "deviation", "percent", "weeks_with_win", "max_week_wins")

	rows := make([]*PersonStat, 0, len(stats))
	for _, name := range names {
		rows = append(rows, stats[name])
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Wins > rows[j].Wins
	})

	for _, stat := range rows {
		if stat.Wins < minWins {
			minWins = stat.Wins
		}
		if stat.Wins > maxWins {
			maxWins = stat.Wins
		}
		diff := float64(stat.Wins) - expected
		if expected > 0 {
			chiSquare += diff * diff / expected
		}
		fmt.Printf("%-16s %-10d %+11.2f%% %9.2f%% %-14d %-14d\n",
			stat.Name,
			stat.Wins,
			percent(diff, expected),
			percent(float64(stat.Wins), float64(totalWinnerSlots)),
			stat.VirtualWeeks,
			stat.MaxWeekWins,
		)
	}

	fmt.Println()
	fmt.Printf("expected_per_person=%.2f\n", expected)
	fmt.Printf("min=%d max=%d range=%d range_rate=%.2f%%\n", minWins, maxWins, maxWins-minWins, percent(float64(maxWins-minWins), expected))
	fmt.Printf("chi_square=%.2f df=%d\n", chiSquare, len(names)-1)
}

func percent(value float64, base float64) float64 {
	if base == 0 {
		return 0
	}
	return value / base * 100
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
