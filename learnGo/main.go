package main

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

import "fmt"

type Number int

func (n *Number) Display() {
	fmt.Println(*n)
}

func (n *Number) Double() {
	*n *= 2
}

func main() {
	number := Number(3)
	pointer := &number
	pointer.Double()
	pointer.Display()
	number.Double()
	number.Double()
	number.Display()
}
