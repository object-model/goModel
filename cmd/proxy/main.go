package main

import (
	"flag"
	"fmt"
	"github.com/object-model/goModel/cmd/proxy/server"
	"io"
	"log"
	"os"
	"time"
)

const Version = "0.0.1"

const Desc = "Proxy is object model proxy server which can transmit model message " +
	"and also provides methods and events itself. " +
	"Model can connect to proxy using tcp or websocket interface."

func main() {
	var webSocket bool
	var webSocketAddr string
	var address string
	var showVersion bool
	var showProxyMeta bool
	var printDataLog bool
	var saveLogFile bool
	flag.BoolVar(&webSocket, "ws", false, "whether to run websocket service")
	flag.StringVar(&webSocketAddr, "wsAddr", "0.0.0.0:9090", "proxy websocket address")
	flag.StringVar(&address, "addr", "0.0.0.0:8080", "proxy tcp address")
	flag.BoolVar(&printDataLog, "p", false, "whether to print send and received message on console")
	flag.BoolVar(&saveLogFile, "log", false, "whether to save send and received message to file")
	flag.BoolVar(&showVersion, "v", false, "show version of proxy and quit")
	flag.BoolVar(&showProxyMeta, "meta", false, "show proxy meta info")

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

	// 打印代理元信息
	if showProxyMeta {
		fmt.Println("proxy meta", server.ProxyMetaString)
	}

	var logWriters []io.Writer

	// 开启控制台打印收发报文
	if printDataLog {
		logWriters = append(logWriters, os.Stdout)
	}

	// 开启记录收发报文到日志
	if saveLogFile {
		// 以当前时间建立日志文件
		_ = os.Mkdir("./logs", os.ModePerm)
		file, err := os.Create(fmt.Sprintf("./logs/%s.log",
			time.Now().Format("[2006-01-02 15.04.05]")))
		if err != nil {
			panic(err)
		}
		defer file.Close()
		logWriters = append(logWriters, file)
	}

	s := server.New(io.MultiWriter(logWriters...))

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
