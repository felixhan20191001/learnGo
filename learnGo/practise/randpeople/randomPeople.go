package main

import (
	"bufio"         // 用于按行读取文件
	"encoding/json" // 用于处理 JSON 数据（前后端通信）
	"fmt"           // 用于打印日志到控制台
	"log"
	"math/rand" // 用于生成随机数
	"net/http"  // 用于搭建 Web 服务器
	"os"        // 用于操作操作系统文件（打开、检查文件）
	"strings"   // 用于处理字符串（去空格、拼接）
	"sync"      // 用于并发控制（互斥锁）
	"time"      // 用于获取当前时间（做随机种子）
)

// --- 全局变量定义 ---

// mu 是互斥锁。
// 作用：因为 Web 服务器是并发的（可以多人同时访问），为了防止多个人同时修改文件导致数据错乱，
// 我们在读写文件时需要“上锁”。
var mu sync.Mutex

// dbFile 是我们要存储名字的文件名
const dbFile = "names.txt"

// --- 数据结构定义 (Model) ---

// Response 用于通用的 API 返回
// 比如：告诉前端操作成功了，或者返回当前的名单列表
/*
通过标签（tag） 控制 JSON 序列化 / 反序列化的行为：
json:"success"：序列化时，字段名Success会转为小写的success（符合 JSON 小写命名习惯）；
json:"msg"：同理，Msg转为msg；
json:"names,omitempty"：
names：字段名转为小写；
omitempty：核心特性—— 如果Names为空切片（[]string{}）或nil，序列化 JSON 时会忽略这个字段，避免返回空的names: []，精简响应数据。*/
type Response struct {
	Success bool     `json:"success"`         // 操作是否成功
	Msg     string   `json:"msg"`             // 提示信息（比如"名字不能为空"）
	Names   []string `json:"names,omitempty"` // 最新的名单列表 (omitempty表示如果为空就不返回这个字段)
}

// DrawResponse 专门用于“抽奖”接口的返回
type DrawResponse struct {
	Winners []string `json:"winners"`         // 中奖的两个人名
	Error   string   `json:"error,omitempty"` // 如果出错（比如人数不足），返回错误信息
}

// ActionRequest 用于接收前端发来的数据
// 比如前端发送 {"name": "张三"}，我们就用这个结构体接收
type ActionRequest struct {
	Name string `json:"name"`
}

// --- 主程序入口 ---

func main() {
	// 1. 初始化随机数种子
	// 如果不加这行，每次重启程序，抽出来的随机结果可能是一样的
	rand.Seed(time.Now().UnixNano())

	// 2. 静态资源服务
	// 告诉 Go：如果用户访问的是普通网址（不是/api开头的），就去当前文件夹找文件（比如 index.html）给用户看,默认先寻找目录下的index.html文件返回
	http.Handle("/", http.FileServer(http.Dir("./")))

	// 3. 注册 API 路由
	// 告诉 Go：当用户访问特定网址时，运行哪个函数
	http.HandleFunc("/api/list", listHandler)  // 获取所有名单
	http.HandleFunc("/api/add", addHandler)    // 新增一个名字
	http.HandleFunc("/api/del", deleteHandler) // 删除一个名字
	http.HandleFunc("/api/draw", drawHandler)  // 开始抽奖

	// 4. 打印启动日志
	fmt.Println("🚀 抽奖系统后端已启动！")
	fmt.Println("📂 数据存储文件:", dbFile)
	fmt.Println("👉 请在浏览器访问: http://localhost:8080")

	// 5. 启动前检查文件
	// 如果 names.txt 不存在，先创建一个空的，防止后面报错
	checkFile()

	// 6. 启动 Web 服务器，监听 8080 端口
	// 这一行代码会一直运行，直到你强制关闭程序
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("启动失败: %v\n", err)
	}
}

// --- 核心工具函数 (Helper Functions) ---

func checkErr(err error) {
	if err != nil {
		log.Printf("出现错误：%s\n", err)
	}
}

// checkFile 检查数据文件是否存在，不存在则创建
func checkFile() {
	// os.Stat 获取文件信息，如果返回 IsNotExist 错误，说明文件不存在
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		fmt.Println("提示: 数据文件不存在，正在创建新文件...")
		// 创建一个空文件
		_, err := os.Create(dbFile)
		checkErr(err)
	}
}

// readNamesFromFile 从 txt 文件中读取所有名字
// 返回值：字符串切片([]string) 和 错误信息(error)
func readNamesFromFile() ([]string, error) {
	// 打开文件
	file, err := os.OpenFile(dbFile, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	// defer 关键字确保函数结束前关闭文件，释放资源
	defer file.Close()

	var names []string
	// 使用 bufio.Scanner 一行一行地读取
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// strings.TrimSpace 去掉每行前后的空格和换行符
		line := strings.TrimSpace(scanner.Text())
		// 如果这行不是空的，就加到列表里
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// writeNamesToFile 把内存里的名字列表写回 txt 文件
func writeNamesToFile(names []string) error {
	// 1. 把切片用换行符 "\n" 拼接成一个长字符串
	// 比如 ["A", "B"] 变成 "A\nB"
	content := strings.Join(names, "\n")

	// 2. 写入文件（覆盖写入）
	// 0666 是文件权限，表示可读可写
	return os.WriteFile(dbFile, []byte(content), 0666)
}

// --- 接口处理函数 (Handlers) ---

// listHandler: 获取名单列表
func listHandler(w http.ResponseWriter, r *http.Request) {
	// 上锁：虽然只是读，但在高并发下，为了保证读到的是完整的数据，建议加锁
	mu.Lock()
	defer mu.Unlock() // 函数结束时自动解锁

	// 读取文件
	names, _ := readNamesFromFile()

	// 把数据打包成 JSON 发给前端
	json.NewEncoder(w).Encode(Response{Success: true, Names: names})
}

// addHandler: 新增名字
func addHandler(w http.ResponseWriter, r *http.Request) {
	// 只允许 POST 请求
	if r.Method != "POST" {
		return
	}

	// 1. 解析前端发来的 JSON 数据
	var req ActionRequest
	json.NewDecoder(r.Body).Decode(&req)

	// 去除空格
	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "名字不能为空"})
		return
	}

	// 2. 关键：上锁！防止并发写入冲突
	mu.Lock()
	defer mu.Unlock()

	// 3. 读取现有名单
	names, _ := readNamesFromFile()

	// 4. 查重：看看名字是不是已经有了
	for _, n := range names {
		if n == newName {
			json.NewEncoder(w).Encode(Response{Success: false, Msg: "名字已存在"})
			return
		}
	}

	// 5. 追加新名字
	names = append(names, newName)

	// 6. 写回文件
	if err := writeNamesToFile(names); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "写入文件失败"})
		return
	}

	// 7. 返回成功信息和最新的名单
	json.NewEncoder(w).Encode(Response{Success: true, Names: names})
}

// deleteHandler: 删除名字
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}

	// 解析前端要删除谁
	var req ActionRequest
	json.NewDecoder(r.Body).Decode(&req)
	target := req.Name

	// 上锁
	mu.Lock()
	defer mu.Unlock()

	// 读取当前名单
	names, _ := readNamesFromFile()

	// 创建一个新切片，用于存放“没被删除”的人
	newNames := make([]string, 0)
	found := false // 标记是否找到了这个人

	// 遍历名单，做过滤
	for _, n := range names {
		if n != target {
			// 如果不是要删的人，就保留
			newNames = append(newNames, n)
		} else {
			// 如果是要删的人，标记一下，且不把它加到 newNames 里
			found = true
		}
	}

	if !found {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "未找到该名字"})
		return
	}

	// 把过滤后的新名单写回文件
	if err := writeNamesToFile(newNames); err != nil {
		json.NewEncoder(w).Encode(Response{Success: false, Msg: "保存文件失败"})
		return
	}

	json.NewEncoder(w).Encode(Response{Success: true, Names: newNames})
}

// drawHandler: 抽奖逻辑
func drawHandler(w http.ResponseWriter, r *http.Request) {
	// 上锁
	mu.Lock()
	defer mu.Unlock()

	// 每次抽奖都重新读取文件，确保是最新的名单
	names, _ := readNamesFromFile()

	// 校验人数
	if len(names) < 2 {
		json.NewEncoder(w).Encode(DrawResponse{Error: "名单中不足2人，无法抽奖！"})
		return
	}

	// --- 抽奖核心算法 ---
	// rand.Perm(N) 会生成一个 0 到 N-1 的随机乱序数组
	// 比如 len=5，Perm 可能生成 [3, 0, 4, 1, 2]
	perm := rand.Perm(len(names))

	// 我们直接取乱序数组的前两个作为索引，去 names 里拿人
	winners := []string{
		names[perm[0]],
		names[perm[1]],
	}

	// 返回中奖者
	json.NewEncoder(w).Encode(DrawResponse{Winners: winners})
}
