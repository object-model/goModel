package main

import (
	"log"
	"proxy/server"
)

func main() {
	s := server.New()

	log.Fatalln(s.ListenServe(":8080"))
}
