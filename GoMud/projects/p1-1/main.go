package main

import "fmt"

func main() {
	price := 87
	paid := 100
	change := paid - price
	fmt.Println("找零:", change, "元")
	n50 := change / 50
	change = change - n50 * 50
	fmt.Println("50元:", n50, "张")
	n20 := change / 20
	change = change - n20 * 20
	fmt.Println("20元:", n20, "张")
	n10 := change / 10
	change = change - n10 * 10
	fmt.Println("10元:", n10, "张")
	n5 := change / 5
	change = change - n5 * 5
	fmt.Println("5元:", n5, "张")
	n1 := change / 1
	change = change - n1 * 1
	fmt.Println("1元:", n1, "张")
}
