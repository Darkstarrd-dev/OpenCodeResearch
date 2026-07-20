package main

import "fmt"

var temp float32 = 40.0

func main(){
	if temp < 36.0 {
		fmt.Println("体温偏低")
	} else if temp <= 37.2 {
		fmt.Println("正常")
	} else if temp <= 38.0 {
		fmt.Println("低烧")
	} else if temp <= 39.0 {
		fmt.Println("中烧")
	} else if temp <= 40.0 {
		fmt.Println("高烧")
	} else {
		fmt.Println("紧急！速就医")
	}

}
