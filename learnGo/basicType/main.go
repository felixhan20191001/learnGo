package main

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
)

func main() {
	var a byte //主要用于存放字符
	a = 'a' + 1
	fmt.Printf("a=%c\n", a)
	b := 97
	fmt.Printf("b=%c\n", b)
	var c rune //用来存放非英文字符
	c = '晗'
	fmt.Printf("c=%c\n", c)

	var d int8 = 100
	var d64 = float64(d)
	fmt.Printf("d64=%g\n", d64)
	fmt.Println("d64 =", d64)

	type I int
	//var i I = 1
	var d1 = I(d) //类型转换比较严格
	fmt.Printf("d1=%d\n", d1)

	//字符串转数字
	var numString = "12"
	myint, err := strconv.Atoi(numString)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("num=%d\n", myint)

	//数字转字符串
	var mi = 32
	fmt.Println(strconv.Itoa(mi))

	//字符串转为float
	fmt.Println(strconv.ParseFloat("1.234", 64))
	f, _ := strconv.ParseFloat("1.234", 64)
	fmt.Println(f)

	//基本类型转字符串
	floatStr := strconv.FormatFloat(1.23479345, 'f', -1, 64)
	fmt.Println("floatStr = ", reflect.TypeOf(floatStr))

}
