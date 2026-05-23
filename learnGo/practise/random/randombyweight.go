package main

import (
	"fmt"
	"net/http"
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

// --- Gin 接口处理函数 ---

func listHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	persons, err := getPerson()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "读取文件失败"})
		return
	}

	names := make([]string, 0, len(persons))
	for _, person := range persons {
		names = append(names, person.Name)
	}
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

	persons, err := getPerson()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "读取名单失败"})
		return
	}
	for _, n := range persons {
		if n.Name == newName {
			c.JSON(http.StatusOK, Response{Success: false, Msg: "名字已存在"})
			return
		}
	}

	newPerson := Person{
		Name:       newName,
		BaseWeight: defaultWeight,
	}
	persons = append(persons, newPerson)
	if err := writePersons(persons); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "写入文件失败"})
		return
	}

	names := make([]string, 0, len(persons))
	for _, person := range persons {
		names = append(names, person.Name)
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

	oldPersons, err := getPerson()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "读取名单失败"})
		return
	}
	oldWeights := make(map[string]int, len(oldPersons))
	for _, person := range oldPersons {
		oldWeights[person.Name] = person.BaseWeight
	}

	// 这里复刻原代码逻辑：全量替换
	re := regexp.MustCompile(`^\d+(\.|、)?\s*`)
	newNames := make([]string, 0)
	newPersons := make([]Person, 0)
	seen := make(map[string]bool)

	// 如果是追加模式，先读旧的
	// currentNames, _ := readNamesFromFile()
	// newNames = append(newNames, currentNames...)

	for _, rawName := range req.Names {
		name := strings.TrimSpace(rawName)
		name = re.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		baseWeight := defaultWeight
		if oldWeight, ok := oldWeights[name]; ok {
			baseWeight = oldWeight
		}
		newPersons = append(newPersons, Person{
			Name:       name,
			BaseWeight: baseWeight,
		})
		newNames = append(newNames, name)
		seen[name] = true
	}

	if err := writePersons(newPersons); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "写入文件失败"})
		return
	}
	_ = writeNamesToFile(newNames)
	c.JSON(http.StatusOK, Response{Success: true, Names: newNames, Msg: "写入文件成功"})
}

func clearHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	emptyNames := make([]Person, 0)
	if err := writePersons(emptyNames); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "清空文件失败"})
		return
	}

	_ = writeNamesToFile([]string{}) //顺便清空names.txt
	c.JSON(http.StatusOK, Response{Success: true, Names: []string{}, Msg: "名单已全部清空"})
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

	persons, err := getPerson()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "读取名单失败"})
		return
	}
	newPersons := make([]Person, 0, len(persons))
	newNames := make([]string, 0, len(persons))
	found := false
	for _, person := range persons {
		if person.Name != target {
			newNames = append(newNames, person.Name)
			newPersons = append(newPersons, person)
		} else {
			found = true
		}
	}
	if !found {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "未找到该名字"})
		return
	}
	if err := writePersons(newPersons); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: "保存文件失败"})
		return
	}
	_ = writeNamesToFile(newNames)
	c.JSON(http.StatusOK, Response{Success: true, Names: newNames})
}

func drawHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()

	DrawNum++
	fmt.Printf("累计抽奖次数: %v\n", DrawNum)

	var req DrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DrawResponse{Error: "数据格式错误"})
		return
	}
	if strings.TrimSpace(req.Operator) == "" {
		c.JSON(http.StatusOK, DrawResponse{Error: "请输入抽奖人姓名"})
		return
	}

	var weightedPersons []WeightedPerson
	weightedPersons = calFinalWeight()
	candidates := make([]WeightedPerson, len(weightedPersons))
	copy(candidates, weightedPersons)

	// 处理抽奖数量，默认为 2
	count := req.Count
	if count <= 0 {
		count = 2
	}

	if len(candidates) < count {
		c.JSON(http.StatusOK, DrawResponse{Error: fmt.Sprintf("名单中不足%d人，无法抽奖！", count)})
		return
	}

	var winners []string
	for i := 0; i < count; i++ {
		index, err := drawOneByWeight(candidates)
		if err != nil {
			c.JSON(http.StatusOK, DrawResponse{Error: "抽奖失败"})
			return
		}
		winners = append(winners, candidates[index].Name)
		candidates = append(candidates[:index], candidates[index+1:]...) //从候选人切片里删除已经中奖的那个人
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
