package main

import "fmt"

func aa() (int, bool) {
	return 1, true
}

var name int = 1

func get2name(name int) {
	name = name + 1
}

// 在同一个 const 块中，每个常量（无论是否显式赋值）都占据一个位置，iota 的值等于当前常量在块中的索引（从 0 开始）。
func main() {
	num, _ := aa() //不分配内存：匿名变量不会占用内存空间，也无法被使用（编译时会报错,忽略返回值：当函数返回多个值但只需部分时，用 _ 忽略无关值。
	fmt.Println(num)
	const (
		P1 = 1.2 + iota // 在 Go 语言的 const 声明块中，iota 的核心特性是每声明一个常量就自动递增 1
		P2
		P3 = "JHA"
		P4 //const 块中，未显式赋值的常量会复用前一个常量的完整赋值表达式（而不是前一个常量的值）
		P5
		P11 = 100
		P6  = iota
	)
	fmt.Println(P1, P2, P3, P4, P5, P11, P6)

	const (
		D1 = iota
	)
	fmt.Println(D1)

	name = 2
	fmt.Println(name)
	get2name(name)
	fmt.Println(name) //函数形参从函数调用中接受的是实参的副本

}

/*
P6 都是 const 块中的第 7 个常量（位置索引 6），
而 iota 的值仅由其在块中的位置索引决定，与其他常量的赋值方式（如 iota+1、字符串、数值等）无关，
因此 P6 的值都是 6。
*/
