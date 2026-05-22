package main

import (
	"fmt"
	"image/color"
	"math"
)

type Point struct {
	X, Y int
}

func (p Point) Distance(q Point) float64 {
	dx := p.X - q.X
	dy := p.Y - q.Y
	return math.Sqrt(float64(dx*dx + dy*dy))
}

func (p *Point) ScaleBy(factor int) {
	p.X *= factor
	p.Y *= factor
}

type ColoredPoint struct {
	*Point
	Color color.RGBA
}

var red = color.RGBA{255, 0, 0, 255}
var blue = color.RGBA{0, 0, 255, 255}

func main() {
	p := ColoredPoint{&Point{1, 1}, red}
	q := ColoredPoint{&Point{5, 4}, blue}
	fmt.Println(p.Distance(*q.Point)) // 输出: 5

	q.Point = p.Point // 共享同一个 Point
	p.ScaleBy(2)
	fmt.Println(*p.Point, *q.Point) // 输出: {2 2} {2 2}
}
