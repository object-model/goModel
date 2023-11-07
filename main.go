package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"proxy/server"
)

const Version = "0.0.1"

const Desc = "proxy is object model proxy server which can transmit model message and also provides methods and events itself."

func main() {
	var address string
	var showVersion bool
	flag.StringVar(&address, "addr", "0.0.0.0:8080", "proxy network address to listen")
	flag.BoolVar(&showVersion, "v", false, "show version of proxy")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Println()
		fmt.Fprintln(flag.CommandLine.Output(), Desc)
	}

	flag.Parse()

	// 显示版本号
	if showVersion {
		fmt.Println("proxy:", Version)
		return
	}

	s := server.New()

	log.Fatalln(s.ListenServe(address))
}
