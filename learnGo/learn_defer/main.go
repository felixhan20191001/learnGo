package main

import "fmt"

func double(x int) (res int) {
	//defer func() { fmt.Printf("defer double %d = %d\n", x, res) }()
	defer fmt.Printf("defer double %d = %d\n", x, res)
	fmt.Printf("double %d = %d\n", x, res)
	return x * x
}
func main() {
	fmt.Println(double(4))
}
