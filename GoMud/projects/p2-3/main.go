package main

import "fmt"

func main(){
	price := 137
	paid := 200
	change := paid - price
	for _, face := range []int{100,50,20,10,5,1}{
		count := change / face
		change %= face
		if count > 0 {
			fmt.Println(count,"张",face,"元")
		}
	}
}
