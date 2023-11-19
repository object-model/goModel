package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"proxy/server"
)

const Version = "0.0.1"

const Desc = "proxy is object model proxy server which can transmit model message " +
	"and also provides methods and events itself."

func main() {
	var webSocket bool
	var webSocketAddr string
	var address string
	var showVersion bool
	flag.BoolVar(&webSocket, "ws", false, "whether or not to run websocket service")
	flag.StringVar(&webSocketAddr, "wsAddr", "0.0.0.0:9090", "proxy websocket address")
	flag.StringVar(&address, "addr", "0.0.0.0:8080", "proxy tcp address")
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

	s := server.New(io.Discard)

	// 开启webSocket服务
	if webSocket {
		go func() {
			fmt.Println("proxy listen websocket at", webSocketAddr)
			log.Fatalln(s.ListenServeWebSocket(webSocketAddr))
		}()
	}

	fmt.Println("proxy listen tcp at", address)
	log.Fatalln(s.ListenServeTCP(address))
}
