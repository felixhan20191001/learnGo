package main

import (
	"fmt"
	"math/rand"
	"unicode/utf8"
)

func awardPrize() {
	switch rand.Intn(3) + 1 {
	case 1:
		fmt.Println("you win a cruise")
	case 2:
		fmt.Println("you win a car")
	case 3:
		fmt.Println("you win a goat")
	default:
		panic("invalid number")

	}
}

func main() {
	//awardPrize()
	asciiString := "abcde"
	utf8String := "我是你爸爸hahah1"
	fmt.Println(utf8.RuneCountInString(asciiString))
	fmt.Println(utf8.RuneCount([]byte(asciiString)))
	fmt.Println(utf8.RuneCountInString(utf8String))
}
