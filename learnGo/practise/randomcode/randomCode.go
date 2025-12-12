package main

import (
	"flag"
	"fmt"
	"math/rand"
)

func main() {
	length := flag.Int("l", 8, "生成的密码长度")
	flag.Parse()

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, *length)

	for i := range password {
		password[i] = charset[rand.Intn(len(charset))]

	}
	fmt.Printf("输出的密码是：%s\n", password)
}
