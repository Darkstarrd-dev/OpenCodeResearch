package main

import "fmt"

func cToF(c float64) float64{
	return c*9/5 + 32
}

func cToK(c float64) float64{
	return c + 273.15
}

func fToC(f float64) float64{
	return (f - 32) / 9 * 5
}

func kToC(k float64) float64{
	return k - 273.15
}

func main(){
	c := 37.0
	fmt.Println(c,"C")
	fmt.Println(cToF(c),"F")
	fmt.Println(cToK(c),"K")
	fmt.Println()
	f := 98.6
	fmt.Println(f,"F")
	fmt.Println(fToC(f),"C")
	fmt.Println(cToK(fToC(f)),"K")
	fmt.Println()
	k := 310.15
	fmt.Println(k,"K")
	fmt.Println(kToC(k),"C")
	fmt.Println(cToF(kToC(k)),"F")


}
