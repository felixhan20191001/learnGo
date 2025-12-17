package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
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

func main() {
	http.Handle("/", http.FileServer(http.Dir("./")))

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
