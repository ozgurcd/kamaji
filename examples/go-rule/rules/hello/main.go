package main

import (
	"flag"
	"fmt"
)

func main() {
	greeting := flag.String("greeting", "Hello", "Greeting to print")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Println(*greeting)
		return
	}
	for _, name := range flag.Args() {
		fmt.Printf("%s, %s!\n", *greeting, name)
	}
}
