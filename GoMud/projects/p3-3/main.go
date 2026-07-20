package main

import "fmt"



func add(a, b float64) float64 {return a+b}   // 返回 a + b
func sub(a, b float64) float64 {return a-b}   // 返回 a - b
func mul(a, b float64) float64 {return a*b}   // 返回 a * b
func div(a, b float64) float64 {return a/b}   // 返回 a / b


func main(){
	x := 7.0
	y := 10.0
	op := "*"
	if op == "+" {
		fmt.Println(x,"+",y,"=",add(x,y))
	} else if op == "-"{
		fmt.Println(x,"-",y,"=",sub(x,y))
	} else if op == "*"{
		fmt.Println(x,"*",y,"=",mul(x,y))
	} else if op == "/"{
		fmt.Println(x,"/",y,"=",div(x,y))
	}

}
