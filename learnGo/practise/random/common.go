package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Person struct {
	Name       string `json:"name"`
	BaseWeight int    `json:"baseWeight"`
}

type LotteryConfig struct {
	TimeWindow int     `json:"timeWindow"` //单位是天
	HitTimes   int     `json:"hitTimes"`
	ReduceRate float64 `json:"reduceRate"`
	MinWeight  int     `json:"minWeight"`
}

type WeightedPerson struct {
	Name           string `json:"name"`
	BaseWeight     int    `json:"baseWeight"`
	RecentWinCount int    `json:"recentWinCount"`
	CurrentWeight  int    `json:"currentWeight"`
}

var defaultConfig = LotteryConfig{
	TimeWindow: 7,
	HitTimes:   2,
	ReduceRate: 0.5,
	MinWeight:  10,
}

const peopleFile = "people.json"
const configFile = "config.json"
const defaultWeight = 100

var cfg, _ = getConfig()

func getConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return configFile
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, configFile)
}

func getConfig() (LotteryConfig, error) {
	filePath := getConfigPath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := setConfig(defaultConfig); err != nil {
			return defaultConfig, err
		}
		return defaultConfig, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return defaultConfig, err
	}
	var config LotteryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return defaultConfig, err
	}
	return config, nil
}

func setConfig(config LotteryConfig) error {
	filePath := getConfigPath()
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0666)
}

func getPersonFile() string {
	exePath, err := os.Executable()
	if err != nil {
		return peopleFile
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, peopleFile)

}

func getPerson() ([]Person, error) {
	filePath := getPersonFile()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return getPersonFromNames()
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	persons := make([]Person, 0)
	if err := json.Unmarshal(data, &persons); err != nil {
		return nil, err
	}
	return persons, nil
}

func getPersonFromNames() ([]Person, error) {
	names, err := readNamesFromFile()
	if err != nil {
		return make([]Person, 0), err
	}
	persons := make([]Person, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		persons = append(persons, Person{
			Name:       name,
			BaseWeight: defaultWeight,
		})
	}
	if err := writePersons(persons); err != nil {
		return nil, err
	}
	return persons, nil
}

func writePersons(persons []Person) error {
	filePath := getPersonFile()
	data, err := json.MarshalIndent(persons, "", "\t") //把 Go 结构体 / 字典等数据，格式化缩进转为 JSON 字符串
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0666)

}

func parseHistoryTime(value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02 15:04:05", v, time.Local)
}

func getCycleStartTime(now time.Time) time.Time {
	weekday := now.Weekday()

	daysSinceMonday := int(weekday - time.Monday)
	if daysSinceMonday < 0 {
		daysSinceMonday = 6
	}
	startTime := now.AddDate(0, 0, -daysSinceMonday)
	//把 startTime 这个时间，保留年月日 **，强制把时分秒毫秒设为 00:00:00，时区用 now 的时区，生成一个新的时间。**
	return time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, now.Location())
}

func countRecentWins(persons []Person, records []HistoryRecord, now time.Time) map[string]int {
	counts := make(map[string]int, len(persons))
	//personSet := make(map[string]bool, len(records))
	for _, person := range persons {
		counts[person.Name] = 0
	}
	startTime := getCycleStartTime(now) //获取这个星期的星期一的零点时间
	for _, record := range records {
		recordTime, err := parseHistoryTime(record.Time)
		if err != nil {
			continue
		}
		if recordTime.Before(startTime) {
			continue
		}
		for _, winner := range record.Winners {
			if _, ok := counts[winner]; ok {
				counts[winner]++
			}
		}
	}
	return counts
}

func stableRank(name string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("lottery-rank-seed:" + strings.TrimSpace(name)))
	return h.Sum64()
}

func calFinalWeight() []WeightedPerson {
	persons, err := getPerson()
	if err != nil {
		fmt.Printf("读取抽奖人员名单失败：%v", err)
		return nil
	}
	counts := countRecentWins(persons, history, time.Now())
	var wp []WeightedPerson

	for _, person := range persons {
		var finalWeight = person.BaseWeight
		if counts[person.Name] > cfg.HitTimes {
			finalWeight = int(
				math.Round(float64(person.BaseWeight) * math.Pow(cfg.ReduceRate, float64(counts[person.Name]-cfg.HitTimes))))
			if finalWeight < cfg.MinWeight {
				finalWeight = cfg.MinWeight
			}
		}
		wp = append(wp, WeightedPerson{
			Name:           person.Name,
			BaseWeight:     person.BaseWeight,
			RecentWinCount: counts[person.Name],
			CurrentWeight:  finalWeight,
		})
	}
	/*
		sort.SliceStable 是 Go 标准库 sort 包的函数，作用是对任意类型的切片做稳定排序：
		第一个参数 wp：待排序的切片（元素需包含 Name 字段，比如抽奖场景的中奖者结构体）；
		第二个参数：匿名比较函数，返回 true 表示「第 i 个元素应该排在第 j 个元素前面」，是排序规则的核心。
	*/
	sort.SliceStable(wp, func(i, j int) bool {
		left := stableRank(wp[i].Name)
		right := stableRank(wp[j].Name)
		if left == right {
			return wp[i].Name < wp[j].Name
		}
		return left < right
	})

	return wp
}
