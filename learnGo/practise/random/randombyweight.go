package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	shuffle "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// --- 全局变量定义 ---
var mu sync.Mutex

const dbFile = "names.txt"
const historyFile = "history.jsonl"
const maxHistoryNum = 100

var DrawNum = 0

// 默认名单
var defaultNames = []string{
	"齐弘宇", "齐宝树", "江龙", "李雪", "刘晓茜", "周成山", "刘先觉",
	"李岷轩", "温嘉鑫", "李亚洲", "张钦", "孟辰", "李亚东",
}

// --- 结构体定义 ---

type HistoryRecord struct {
	Time     string   `json:"time"`
	Operator string   `json:"operator"`
	Winners  []string `json:"winners,omitempty"`
}

type DrawRequest struct {
	Operator string `json:"operator"`
	Count    int    `json:"count"` // 支持抽奖人数
}

var history []HistoryRecord

type Response struct {
	Success bool     `json:"success"`
	Msg     string   `json:"msg"`
	Names   []string `json:"names,omitempty"`
}

type DrawResponse struct {
	Winners []string `json:"winners"`
	Error   string   `json:"error,omitempty"`
}

type ActionRequest struct {
	Name string `json:"name"`
}

type BatchActionRequest struct {
	Names []string `json:"names"`
}

// --- 主程序入口 ---

func main() {
	// 初始化数据
	if err := initData(); err != nil {
		fmt.Printf("数据初始化失败，%v\n", err)
		return
	}

	his, err := readHistoryFromFile()
	if err != nil {
		fmt.Printf("从jsonl中获取历史结果失败%v\n", err)
		history = make([]HistoryRecord, 0)
	} else {
		history = his
	}

	// 设置 Gin 模式 (ReleaseMode 可以隐藏调试日志)
	// gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// 1. API 路由组
	api := r.Group("/api")
	{
		api.GET("/list", listHandler)
		api.POST("/add", addHandler)
		api.POST("/del", deleteHandler)
		api.POST("/draw", drawHandler)
		api.GET("/history", historyHandler)
		api.POST("/batch_add", batchAddHandler)
		api.POST("/clear", clearHandler)
	}

	// 2. 静态资源服务 (模拟 http.FileServer(http.Dir("./")))
	// 使用 NoRoute 可以匹配所有未被 API 捕获的路径，比如 /, /index.html, /image_0.jpg
	r.NoRoute(gin.WrapH(http.FileServer(http.Dir("."))))

	fmt.Println("🚀 抽奖系统后端已启动 (Gin版)！")
	fmt.Println("📂 数据存储文件:", getDBPath())
	fmt.Println("👉 请在浏览器访问: http://localhost:8181")

	// 3. 启动服务
	if err := r.Run(":8181"); err != nil {
		fmt.Printf("启动失败: %v\n", err)
	}
}

// --- 工具函数 ---

// 获取可执行文件所在的目录
func getExecDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// getDBPath 获取 names.txt 的绝对路径
func getDBPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return dbFile
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, dbFile)
}

func getHistoryPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return historyFile
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, historyFile)
}

func initData() error {
	filePath := getDBPath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return writeNamesToFile(defaultNames)
	}
	return nil
}

func readNamesFromFile() ([]string, error) {
	filePath := getDBPath()
	file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	names := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func writeNamesToFile(names []string) error {
	content := strings.Join(names, "\n")
	filePath := getDBPath()
	return os.WriteFile(filePath, []byte(content), 0666)
}

func readHistoryFromFile() ([]HistoryRecord, error) {
	filePath := getHistoryPath()
	file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	history := make([]HistoryRecord, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record HistoryRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		history = appendRollingHistory(history, record)
	}
	return history, scanner.Err()
}

func addHistoryToJsonl(result HistoryRecord) error {
	filepath := getHistoryPath()
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = file.WriteString(string(data) + "\n")
	return err
}

func appendRollingHistory(records []HistoryRecord, record HistoryRecord) []HistoryRecord {
	if len(records) < maxHistoryNum {
		return append(records, record)
	}
	copy(records, records[1:])
	records[len(records)-1] = record
	return records
}

func shuffleSlice[T any](slice []T, num int) {
	for i := 0; i <= num; i++ {
		shuffle.Shuffle(len(slice), func(i, j int) {
			slice[i], slice[j] = slice[j], slice[i]
		})
	}
}

// --- Gin 接口处理函数 ---

func listHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	names, _ := readNamesFromFile()
	c.JSON(http.StatusOK, Response{Success: true, Names: names})
}

func addHandler(c *gin.Context) {
	var req ActionRequest
	// ShouldBindJSON 自动解析 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "参数错误"})
		return
	}

	//移除字符串首尾的所有空白字符
	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "名字不能为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	names, _ := readNamesFromFile()
	for _, n := range names {
		if n == newName {
			c.JSON(http.StatusOK, Response{Success: false, Msg: "名字已存在"})
			return
		}
	}

	names = append(names, newName)
	if err := writeNamesToFile(names); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "写入文件失败"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Names: names})
}

func batchAddHandler(c *gin.Context) {
	var req BatchActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "数据格式错误"})
		return
	}

	if len(req.Names) == 0 {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "名单为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 重新读取当前文件内容，避免覆盖（如果你想保留旧数据）
	// 或者如果你想完全覆盖，就保持你原来的逻辑。
	// 这里保留你原来的逻辑：先清空，再正则处理，再写入。
	// 但通常批量添加是 append，这里按你原代码逻辑可能是覆盖或添加。
	// 原代码：读取 req.Names -> 去重/格式化 -> 写入。
	// 原代码逻辑似乎是：clearFile() 然后写入。我们保持一致。

	// 注意：你原来的 batchAddHandler 里面调用了 clearFile()，这意味着是“覆盖导入”。
	// 如果你想改为“追加导入”，请去掉 clearFile 并读取 currentNames := readNamesFromFile()

	// 这里复刻原代码逻辑：全量替换
	re := regexp.MustCompile(`^\d+(\.|、)?\s*`)
	newNames := make([]string, 0)

	// 如果是追加模式，先读旧的
	// currentNames, _ := readNamesFromFile()
	// newNames = append(newNames, currentNames...)

	for _, rawName := range req.Names {
		name := strings.TrimSpace(rawName)
		name = re.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		if name != "" {
			newNames = append(newNames, name)
		}
	}

	if err := writeNamesToFile(newNames); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "写入文件失败"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Names: newNames, Msg: "写入文件成功"})
}

func clearHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	emptyNames := make([]string, 0)
	if err := writeNamesToFile(emptyNames); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "清空文件失败"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Names: emptyNames, Msg: "名单已全部清空"})
}

func deleteHandler(c *gin.Context) {
	var req ActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "参数错误"})
		return
	}
	target := req.Name

	mu.Lock()
	defer mu.Unlock()
	names, _ := readNamesFromFile()
	newNames := make([]string, 0)
	found := false
	for _, n := range names {
		if n != target {
			newNames = append(newNames, n)
		} else {
			found = true
		}
	}
	if !found {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "未找到该名字"})
		return
	}
	if err := writeNamesToFile(newNames); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "保存文件失败"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Names: newNames})
}

func drawHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	//DrawNum++
	//fmt.Printf("累计抽奖次数: %v\n", DrawNum)

	var req DrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DrawResponse{Error: "数据格式错误"})
		return
	}
	if strings.TrimSpace(req.Operator) == "" {
		c.JSON(http.StatusOK, DrawResponse{Error: "请输入抽奖人姓名"})
		return
	}

	names, _ := readNamesFromFile()

	// 处理抽奖数量，默认为 2
	count := req.Count
	if count <= 0 {
		count = 2
	}

	if len(names) < count {
		c.JSON(http.StatusOK, DrawResponse{Error: fmt.Sprintf("名单中不足%d人，无法抽奖！", count)})
		return
	}

	candidates := make([]string, len(names))
	copy(candidates, names)
	shuffleSlice(candidates, 5)

	var winners []string
	for i := 0; i < count; i++ {
		currentLen := len(candidates)
		bigIdx, err := rand.Int(rand.Reader, big.NewInt(int64(currentLen)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, DrawResponse{Error: "随机数生成器故障"})
			return
		}
		idx := int(bigIdx.Int64())
		winners = append(winners, candidates[idx])
		candidates[idx] = candidates[currentLen-1]
		candidates = candidates[:currentLen-1]
	}

	record := HistoryRecord{
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Operator: req.Operator,
		Winners:  winners,
	}
	history = appendRollingHistory(history, record)
	if err := addHistoryToJsonl(record); err != nil {
		fmt.Printf("抽奖结果写入文件失败：%v", err)
	}
	c.JSON(http.StatusOK, DrawResponse{Winners: winners})
}

func historyHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	if history == nil {
		history = make([]HistoryRecord, 0)
	}
	c.JSON(http.StatusOK, history)
}
