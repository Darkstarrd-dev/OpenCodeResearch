package main

import "fmt"

func main() {
	c1 := 37.0
	f1 := c1*9/5 + 32
	k1 := c1 + 273.15
	fmt.Println("源:", c1, "C")
	fmt.Println("  →", f1, "F")
	fmt.Println("  →", k1, "K")

	f2 := 98.6
	c2 := (f2 - 32) * 5 / 9
	k2 := c2 + 273.15
	fmt.Println("源:", f2, "F")
	fmt.Println("  →", c2, "C")
	fmt.Println("  →", k2, "K")

	k3 := 310.15
	c3 := k3 - 273.15
	f3 := c3*9/5 + 32
	fmt.Println("源:", k3, "K")
	fmt.Println("  →", c3, "C")
	fmt.Println("  →", f3, "F")
}
