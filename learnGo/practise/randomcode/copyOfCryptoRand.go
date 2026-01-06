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
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex

const dbFile = "names.txt"

// 全局变量必须用var声明
var defaultNames = []string{
	"齐弘宇",
	"齐宝树",
	"江龙",
	"李雪",
	"刘晓茜",
	"周成山",
	"刘先觉",
	"李岷轩",
	"温嘉鑫",
	"李亚洲",
	"张钦",
	"孟辰",
	"李亚东",
}

type HistoryRecord struct {
	Time     string   `json:"time"`
	Operator string   `json:"operator"`
	Winners  []string `json:"winners,omitempty"`
}

type DrawRequset struct {
	Operator string ``
}

type Response struct {
	Success bool     `json:"success"`
	Msg     string   `json:"msg"`
	Names   []string `json:"names,omitempty"`
}

type DrawResponse struct {
	Winners []string `json:"winners"`
	Error   error    `json:"error,omitempty"`
}

type ActionRequest struct {
	Name string `json:"name"`
}

var history []HistoryRecord

func main() {
	http.Handle("/", http.FileServer(http.Dir("./")))

	http.HandleFunc("api/list", listHandler)
	http.HandleFunc("api/add", addHandler)
	http.HandleFunc("api/del", deleteHandler)
	http.HandleFunc("api/draw", drawHandler)
	http.HandleFunc("api/history", historyHandler)

	fmt.Println("🚀 抽奖系统后端已启动！")
	fmt.Println("📂 数据存储文件: ", dbFile)
	fmt.Println("请在浏览器访问 http://localhost:8181")

	if err := initData(); err != nil {
		fmt.Printf("数据初始化失败，%v\n", err)
		return
	}

	if err := http.ListenAndServe(":8181", nil); err != nil {
		fmt.Printf("启动失败： %v\n", err)
	}

}

func checkErr(err error) {
	if err != nil {
		log.Printf("出现错误：%v\n", err)
	}
}

func checkFile() {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		_, err := os.Create(dbFile)
		checkErr(err)
	}
}

func readNameFromFile() ([]string, error) {
	file, err := os.OpenFile(dbFile, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var names []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

func writeNameToFile(names []string) error {
	content := strings.Join(names, "\n")
	return os.WriteFile(dbFile, []byte(content), 0666)
}

func initData() error {
	return writeNameToFile(defaultNames)
}

func shuffleSlice[T any](slice []T, num int) {
	for i := 0; i < num; i++ {
		shuffle.Shuffle(len(slice), func(i, j int) {
			slice[i], slice[j] = slice[j], slice[i]
		})
	}
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	names, _ := readNameFromFile()
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

	names, _ := readNameFromFile()
	for _, name := range names {
		if name == newName {
			json.NewEncoder(w).Encode(Response{Success: false, Msg: "名字已存在"})
			return
		}
	}

	names = append(names, newName)
	if err := writeNameToFile(names); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "写入文件失败"})
		return
	}

	json.NewEncoder(w).Encode(Response{Success: true, Names: names})
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

	names, _ := readNameFromFile()
	newNames := make([]string, 0)
	found := false

	for _, n := range names {
		if n != target {
			newNames = append(newNames, n)
		} else {
			found = true
		}
	}

	if found == false {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "未找到改名字"})
		return
	}

	if err := writeNameToFile(newNames); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "保存文件失败"})
		return
	}

	json.NewEncoder(w).Encode(Response{Success: true, Names: newNames})
}

func drawHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	var req DrawRequset
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "数据格式错误"})
		return
	}

	if strings.TrimSpace(req.Operator) == "" {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "请输入抽奖人姓名"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	names, _ := readNameFromFile()
	if len(names) < 2 {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "名单不足两人，无法抽奖"})
		return
	}

	candidatas := make([]string, len(names))
	copy(candidatas, names)
	shuffleSlice(candidatas, 5)
	fmt.Printf("打乱名单后的结果：%v\n", candidatas)

	var winners []string
	for i := 0; i < 2; i++ {
		curLen := len(candidatas)
		bigIdx, err := rand.Int(rand.Reader, big.NewInt(int64(curLen)))
		if err != nil {
			json.NewEncoder(w).Encode(Response{Success: false, Msg: "随机数生成器故障"})
			return
		}

		idx := int(bigIdx.Int64())
		winners = append(winners, candidatas[idx])

		candidatas[idx] = candidatas[curLen-1]
		candidatas = candidatas[0 : curLen-1]
	}

	record := HistoryRecord{
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Operator: req.Operator,
		Winners:  winners,
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
