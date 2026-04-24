package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	shuffle "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// --- 全局变量定义 ---
var mu sync.Mutex

const dbFile = "names.txt"
const historyFile = "history.jsonl"

var DrawNum = 0

// 默认名单（当文件不存在时使用）
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
	Count    int    `json:"count"`
}

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

var history []HistoryRecord

// --- 主程序入口 ---

func main() {
	// 1. 静态资源服务
	http.Handle("/", http.FileServer(http.Dir("./")))

	// 2. 注册 API 路由 (请确保这里都有)
	http.HandleFunc("/api/list", listHandler)
	http.HandleFunc("/api/add", addHandler)
	http.HandleFunc("/api/del", deleteHandler)
	http.HandleFunc("/api/draw", drawHandler)
	http.HandleFunc("/api/history", historyHandler)
	http.HandleFunc("/api/batch_add", batchAddHandler) // 批量导入
	http.HandleFunc("/api/clear", clearHandler)        // 【关键】必须注册这个清空接口！

	fmt.Println("🚀 抽奖系统后端已启动！")
	fmt.Println("📂 数据存储文件:", dbFile)
	fmt.Println("👉 请在浏览器访问: http://localhost:8181")

	// 3. 初始化数据
	if err := initData(); err != nil {
		fmt.Printf("数据初始化失败，%v\n", err)
		return
	}

	his, err := readHistoryFromFile()
	if err != nil {
		fmt.Printf("获取抽奖历史失败: %v\n", err)
		history = make([]HistoryRecord, 0)
	} else {
		history = his
	}

	// 4. 启动服务
	if err := http.ListenAndServe(":8181", nil); err != nil {
		fmt.Printf("启动失败: %v\n", err)
	}
}

// --- 工具函数 ---

func getDBPath() string {
	// runtime.Caller(0) 获取当前调用函数的文件位置
	// filename 就是这个 randombyweight.go 文件的完整绝对路径
	//_, filename, _, ok := runtime.Caller(0)
	//if !ok {
	//	return dbFile
	//}
	//dir := filepath.Dir(filename)
	//return filepath.Join(dir, "names.txt")

	// 获取当前执行程序的绝对路径 (例如 /Users/.../randpeople1/battery)
	exePath, err := os.Executable()
	if err != nil {
		// 极端情况获取失败，回退到相对路径
		return dbFile
	}

	// 获取目录 (例如 /Users/.../randpeople1)
	dir := filepath.Dir(exePath)

	// 拼接完整路径 (例如 /Users/.../randpeople1/names.txt)
	return filepath.Join(dir, dbFile)
}

func getHistoryPath() string {

	//runtime.Caller(0) //获取当前调用函数的文件位置
	////filename 就是这个 randombyweight.go 文件的完整绝对路径
	//_, filename, _, ok := runtime.Caller(0)
	//if !ok {
	//	return historyFile
	//}
	//dir := filepath.Dir(filename)
	//return filepath.Join(dir, historyFile)

	exePath, err := os.Executable()
	if err != nil {
		// 极端情况获取失败，回退到相对路径
		return historyFile
	}

	// 获取目录 (例如 /Users/.../randpeople1)
	dir := filepath.Dir(exePath)

	// 拼接完整路径 (例如 /Users/.../randpeople1/names.txt)
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
	//fmt.Println(filePath)
	file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 初始化为空切片而不是 nil，防止 JSON 序列化为 null
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

func readHistoryFromFile() ([]HistoryRecord, error) {
	filePath := getHistoryPath()
	file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []HistoryRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record HistoryRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, scanner.Err()
}

func addHistory(record HistoryRecord) error {
	filePath := getHistoryPath()
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = file.WriteString(string(data) + "\n") //空白标识符不算新变量，err之前已经声明过，所有这一行不需要重新声明
	return err
}

func clearFile() error {
	emptyNames := make([]string, 0)

	// 写入文件（覆盖现有内容）
	if err := writeNamesToFile(emptyNames); err != nil {
		log.Println("清空文件失败")
		return err
	}
	return nil
}

func writeNamesToFile(names []string) error {
	content := strings.Join(names, "\n")
	filePath := getDBPath()
	//fmt.Println(filePath)
	return os.WriteFile(filePath, []byte(content), 0666)
}

func shuffleSlice[T any](slice []T, num int) {
	for i := 0; i <= num; i++ {
		shuffle.Shuffle(len(slice), func(i, j int) {
			slice[i], slice[j] = slice[j], slice[i]
		})
	}
}

// --- 接口处理函数 ---

func listHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	names, _ := readNamesFromFile()
	json.NewEncoder(w).Encode(Response{Success: true, Names: names})
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	var req ActionRequest
	json.NewDecoder(r.Body).Decode(&req)

	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "名字不能为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	names, _ := readNamesFromFile()
	for _, n := range names {
		if n == newName {
			json.NewEncoder(w).Encode(Response{Success: false, Msg: "名字已存在"})
			return
		}
	}

	names = append(names, newName)
	if err := writeNamesToFile(names); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "写入文件失败"})
		return
	}
	json.NewEncoder(w).Encode(Response{Success: true, Names: names})
}

func batchAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	var req BatchActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "数据格式错误"})
		return
	}

	if len(req.Names) == 0 {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "名单为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	clearFile()
	re := regexp.MustCompile(`^\d+(\.|、)?\s*`)

	currentNames := make([]string, 0)
	for _, rawName := range req.Names {
		name := strings.TrimSpace(rawName)
		name = re.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		currentNames = append(currentNames, name)
	}
	if err := writeNamesToFile(currentNames); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "写入文件失败"})
		return
	}
	json.NewEncoder(w).Encode(Response{Success: true, Names: currentNames, Msg: "写入文件成功"})
}

// 【清空功能】
func clearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 创建一个空切片
	emptyNames := make([]string, 0)

	// 写入文件（覆盖现有内容）
	if err := writeNamesToFile(emptyNames); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "清空文件失败"})
		return
	}

	json.NewEncoder(w).Encode(Response{Success: true, Names: emptyNames, Msg: "名单已全部清空"})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	var req ActionRequest
	json.NewDecoder(r.Body).Decode(&req)
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
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "未找到该名字"})
		return
	}
	if err := writeNamesToFile(newNames); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "保存文件失败"})
		return
	}
	json.NewEncoder(w).Encode(Response{Success: true, Names: newNames})
}

func drawHandler(w http.ResponseWriter, r *http.Request) {
	//DrawNum = DrawNum + 1
	//fmt.Printf("累计抽奖次数: %v\n", DrawNum)
	if r.Method != "POST" {
		return
	}
	var req DrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(DrawResponse{Error: "数据格式错误"})
		return
	}
	if strings.TrimSpace(req.Operator) == "" {
		json.NewEncoder(w).Encode(DrawResponse{Error: "请输入抽奖人姓名"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	names, _ := readNamesFromFile()

	drawCount := req.Count
	if drawCount <= 0 {
		drawCount = 2
	}

	if len(names) < drawCount {
		Msg := fmt.Sprintf("名单中不足 %d 人，无法抽奖！", drawCount)
		json.NewEncoder(w).Encode(DrawResponse{Error: Msg})
		return
	}

	candidates := make([]string, len(names))
	copy(candidates, names)
	shuffleSlice(candidates, 5)

	var winners []string
	for i := 0; i < drawCount; i++ {
		currentLen := len(candidates)
		bigIdx, err := rand.Int(rand.Reader, big.NewInt(int64(currentLen)))
		if err != nil {
			json.NewEncoder(w).Encode(DrawResponse{Error: "随机数生成器故障"})
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
	if err := addHistory(record); err != nil {
		fmt.Printf("抽奖历史写入文件失败：%v", err)
	}
	history = append(history, record)
	json.NewEncoder(w).Encode(DrawResponse{Winners: winners})
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if history == nil {
		history = make([]HistoryRecord, 0)
	}
	json.NewEncoder(w).Encode(history)
}
