package main

import "fmt"

func main() {
	name := "勇者"
	hp := 100
	maxhp := 100
	level := 1
	exp := 0

	fmt.Printf("===== 角色卡 =====\n")
	fmt.Printf("姓名: %s\n", name)
	fmt.Printf("等级: %d\n", level)
	fmt.Printf("经验: %d/100\n", exp)
	fmt.Printf("生命: %d/%d\n", hp, maxhp)
	fmt.Printf("=================\n")
}
