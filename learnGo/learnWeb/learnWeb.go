package main

import (
	"log"
	"net/http"
)

func viewHandler(writer http.ResponseWriter, req *http.Request) {
	//mes := []byte("Hello web") 72 101 108 108 111 32 119 101 98
	mes := []byte{72, 101, 108, 108, 111, 32, 119, 101, 98}
	_, err := writer.Write(mes)
	if err != nil {
		log.Fatal(err)
	}
}

// 向响应写入消息
func write(writer http.ResponseWriter, message string) {
	_, err := writer.Write([]byte(message))
	if err != nil {
		log.Fatal(err)
	}
}

// 处理路由d的请求
func d(writer http.ResponseWriter, request *http.Request) {
	write(writer, "z")
}

// 处理路由e的请求
func e(writer http.ResponseWriter, request *http.Request) {
	write(writer, "x")
}

// 处理路由f的请求
func f(writer http.ResponseWriter, request *http.Request) {
	write(writer, "y")
}

func main() {
	//http.HandleFunc("/hello", viewHandler)
	//err := http.ListenAndServe("localhost:8080", nil)
	//log.Fatal(err)
	http.HandleFunc("/a", d)
	http.HandleFunc("/b", e)
	http.HandleFunc("/c", f)
	e := http.ListenAndServe("localhost:4567", nil)
	log.Fatal(e)

}
