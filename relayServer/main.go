// relayServer, for built simplification, this file contains all method and not being split into packages.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
)

type Config struct {
	ListenHost    string
	ListenPort    int
	NetBufferSize int
	MaxRetries    int
}

var (
	GlobalConfig = Config{}

	listenHost = flag.String("host", "127.0.0.1", "Listen host")
	listenPort = flag.Int("port", 10000, "Listen port")
)

// Initializing
func Init() {
	GlobalConfig = Config{
		ListenHost:    *listenHost,
		ListenPort:    *listenPort,
		NetBufferSize: 1024,
		MaxRetries:    5,
	}

	log.Printf("Listening on Host: [%s:%d]", *listenHost, *listenPort)
}

// Check error wrapper
func CheckError(err error) {
	if err == nil {
		return
	}

	log.Printf("Error occuried: [%v]\n", err)
	panic(1)
}

func relayServer() {

	addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, GlobalConfig.ListenPort)

	l, err := net.Listen("tcp", addr)
	CheckError(err)

	defer l.Close()

	log.Println("relayServer is running...")

	for {
		connServer, err := l.Accept()
		CheckError(err)

		remoteAddr := connServer.RemoteAddr()
		log.Println("relay connected, from addr: ", remoteAddr.String())

		go clientServer(connServer)
	}
}

func clientServer(connServer net.Conn) {

	addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, 20000)

	l, err := net.Listen("tcp", addr)
	CheckError(err)

	defer l.Close()

	log.Println("clientServer is running...")

	for {
		connClient, err := l.Accept()
		CheckError(err)

		remoteAddr := connClient.RemoteAddr()
		log.Println("client connected, from addr: ", remoteAddr.String())

		go clientHandler(connClient, connServer)
	}
}

func netMixer(conn1 net.Conn, conn2 net.Conn) {

	input := bufio.NewScanner(conn1)
	input.Split(bufio.ScanBytes)

	for input.Scan() {
		conn2.Write(input.Bytes())
	}
}

func clientHandler(connClient net.Conn, connServer net.Conn) {

	var (
		wg sync.WaitGroup
	)

	go netMixer(connServer, connClient)
	go netMixer(connClient, connServer)

	wg.Add(1)

	wg.Wait()

	connClient.Close()

	return
}

// Main function
func main() {
	Init()
	relayServer()
}
