package main

import (
	"bufio"         // 用于按行读取文件
	"crypto/rand"   // 注意：这里改用了 crypto/rand
	"encoding/json" // 用于处理 JSON 数据（前后端通信）
	"fmt"           // 用于打印日志到控制台
	"log"
	"math/big" // 用于生成随机数
	shuffle "math/rand/v2"
	"net/http" // 用于搭建 Web 服务器
	"os"       // 用于操作操作系统文件（打开、检查文件）
	"strings"  // 用于处理字符串（去空格、拼接）
	"sync"     // 用于并发控制（互斥锁）
	"time"
)

// --- 全局变量定义 ---

// mu 是互斥锁。
// 作用：因为 Web 服务器是并发的（可以多人同时访问），为了防止多个人同时修改文件导致数据错乱，
// 我们在读写文件时需要“上锁”。
var mu sync.Mutex

// dbFile 是我们要存储名字的文件名
const dbFile = "names.txt"

// 初始化候选人名单
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

// --- 新增结构体 ---

// HistoryRecord 用于记录每一次抽奖的历史

type HistoryRecord struct {
	Time     string   `json:"time"`
	Operator string   `json:"operator"`
	Winners  []string `json:"winners,omitempty"`
}

// DrawRequest 专门用于接收抽奖请求（包含操作人名字）
type DrawRequest struct {
	Operator string `json:"operator"`
}

// --- 全局变量 ---

// history 存储本次启动后的所有抽奖记录
var history []HistoryRecord

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
	//rand.Seed(time.Now().UnixNano())

	// 2. 静态资源服务
	// 告诉 Go：如果用户访问的是普通网址（不是/api开头的），就去当前文件夹找文件（比如 index11.html）给用户看,默认先寻找目录下的index.html文件返回
	http.Handle("/", http.FileServer(http.Dir("./")))

	// 3. 注册 API 路由
	// 告诉 Go：当用户访问特定网址时，运行哪个函数
	http.HandleFunc("/api/list", listHandler)  // 获取所有名单
	http.HandleFunc("/api/add", addHandler)    // 新增一个名字
	http.HandleFunc("/api/del", deleteHandler) // 删除一个名字
	http.HandleFunc("/api/draw", drawHandler)  // 开始抽奖
	http.HandleFunc("/api/history", historyHandler)

	// 4. 打印启动日志
	fmt.Println("🚀 抽奖系统后端已启动！")
	fmt.Println("📂 数据存储文件:", dbFile)
	fmt.Println("👉 请在浏览器访问: http://localhost:8181")

	// 5. 启动前检查文件
	// 如果 names.txt 不存在，先创建一个空的，防止后面报错
	//checkFile()

	//初始化名单每次写入文件
	if err := initData(); err != nil {
		fmt.Printf("数据初始化失败，%v\n", err)
		return
	}
	// 6. 启动 Web 服务器，监听 8080 端口
	// 这一行代码会一直运行，直到你强制关闭程序
	if err := http.ListenAndServe(":8181", nil); err != nil {
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

// // 【新增/修改】初始化数据函数
// // 每次启动时，都把 defaultNames 写入文件，覆盖之前的旧数据
func initData() error {
	return writeNamesToFile(defaultNames)
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

// 切片打乱函数
func shuffleSlice[T any](slice []T, num int) { // 泛型说明：[T any] 支持任意类型切片（int/string/自定义结构体等）
	for i := 0; i <= num; i++ {
		shuffle.Shuffle(len(slice), func(i, j int) {
			slice[i], slice[j] = slice[j], slice[i]
		})
	}
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

// drawHandler: 增强版抽奖逻辑
func drawHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	// 1. 解析前端传来的“抽奖人”名字
	var req DrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(DrawResponse{Error: "数据格式错误"})
		log.Println("数据格式错误")
		return
	}
	// 2. 校验：抽奖人必须填名字
	if strings.TrimSpace(req.Operator) == "" {
		json.NewEncoder(w).Encode(DrawResponse{Error: "请输入抽奖人姓名"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	names, _ := readNamesFromFile()

	// 1. 校验人数
	if len(names) < 2 {
		json.NewEncoder(w).Encode(DrawResponse{Error: "名单中不足2人，无法抽奖！"})
		return
	}

	// --- 增强版抽奖核心算法 ---

	// 我们需要抽取 2 个中奖者。
	// 为了保证绝对随机且不重复，我们模拟从箱子里“拿出一个，扔掉，再拿下一个”的过程。

	// 复制一份名单，以免修改原始切片顺序（虽然这里修改也没事，但在复杂系统中是好习惯）
	candidates := make([]string, len(names))
	copy(candidates, names)

	//打乱5次
	shuffleSlice(candidates, 5)
	fmt.Printf("打乱后的名单是：%v\n", candidates)

	var winners []string

	// 循环 2 次，抽取 2 个人
	for i := 0; i < 2; i++ {
		// currentLen 是当前剩余的候选人数
		currentLen := len(candidates)
		//fmt.Printf("抽两次人，第 %d 轮抽当前候选人数： %d\n", i, currentLen)

		// 生成一个 [0, currentLen) 范围内的真随机数
		// crypto/rand 生成的是 *big.Int，需要转换
		bigIdx, err := rand.Int(rand.Reader, big.NewInt(int64(currentLen)))
		if err != nil {
			// 极罕见情况：操作系统随机源出错
			json.NewEncoder(w).Encode(DrawResponse{Error: "随机数生成器故障"})
			return
		}

		// 拿到随机索引
		idx := int(bigIdx.Int64())

		// 1. 选中这个人，加入中奖名单
		winners = append(winners, candidates[idx])

		// 2. 从候选名单中移除这个人，防止被重复抽中
		// 技巧：把选中的元素和切片最后一个元素“交换”，然后把切片长度减 1
		// 这样不仅效率高（O(1)），而且避免了数组整体移动
		candidates[idx] = candidates[currentLen-1]
		candidates = candidates[:currentLen-1]
	}
	// 4. 【关键】记录历史
	record := HistoryRecord{
		Time:     time.Now().Format("2006-01-02 15:04:05"), //固定格式展示日期
		Operator: req.Operator,
		Winners:  winners,
	}

	history = append(history, record)
	// 返回中奖者
	log.Printf("抽取的获奖者是： %s\n", winners)
	json.NewEncoder(w).Encode(DrawResponse{Winners: winners})
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// 直接返回内存里的 history 切片
	// 为了防止前端处理 null，如果 history 是 nil，初始化为空切片
	if history == nil {
		history = make([]HistoryRecord, 0)
	}

	json.NewEncoder(w).Encode(history)
}
