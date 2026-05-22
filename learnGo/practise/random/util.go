package main

import (
	"bufio"
	"encoding/json"
	shuffle "math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

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
