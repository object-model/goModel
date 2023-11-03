package main

import (
	"fmt"
	"log"
	"proxy/server"
)

func main() {
	fmt.Println("hello proxy")

	s := server.New()

	log.Fatalln(s.ListenServe(":8080"))
}
