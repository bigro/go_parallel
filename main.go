package main

import (
	"fmt"
	"time"
)
var data int

func main() {
	increment()

	memmoryAccsess.Lock()
	if data == 0 {
		fmt.Printf("the value is 0. \n")
	} else {
		fmt.Printf("the value is %v. \n", data)
	}

	memmoryAccsess.Unlock()
}
