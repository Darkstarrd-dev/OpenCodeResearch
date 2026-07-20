package main

import "fmt"

func isPrime(n int) bool {
	for i:=2;i<n-1;i++{
		if n % i == 0 {
			return false
		}
	}
	return true
}

func main(){
	for _, n := range []int{2,3,4,5,6,7,8,9,10,11,13,17,19,20}{
		condition := isPrime(n)
		fmt.Println(n," → ",condition)
	}
}
