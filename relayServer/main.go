package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

type Config struct {
	ListenHost    string
	ListenPort    int
	NetBufferSize int
	MaxRetries    int
}

type Pool struct {
	ArPorts  []int
	NextPort int
}

var GlobalConfig = Config{}
var mutexPortsPool = &sync.Mutex{}
var pool = Pool{}

func Init() {
	GlobalConfig = Config{
		ListenHost:    "127.0.0.1",
		ListenPort:    10000,
		NetBufferSize: 1024,
		MaxRetries:    5,
	}

	pool = Pool{
		ArPorts:  make([]int, 65535),
		NextPort: 20000,
	}
}

func CheckError(err error) {
	if err == nil {
		return
	}

	log.Printf("Error occuried: [%v]\n", err)
	os.Exit(1)
}

func main() {

	Init()
	startServer()
}

func startServer() {
	addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, GlobalConfig.ListenPort)

	l, err := net.Listen("tcp", addr)
	CheckError(err)

	defer l.Close()

	log.Println("Server is running...")
	for {
		conn, err := l.Accept()
		CheckError(err)

		log.Println("...connection attempt")
		netQueue := make(chan []byte)

		go relayClientHandler(conn, netQueue)
	}
}

func relayClientHandler(c net.Conn, nq chan []byte) {

	log.Println("relayClientHandler\t spawned")

	go relayServer(nq)

	buf := make([]byte, GlobalConfig.NetBufferSize)
	for {
		select {
		case out := <-nq:
			n, err := c.Write(out)
			log.Printf("relayClientHandler\t Write: [%d]", n)
			if err != nil {
				log.Printf("relayClientHandler\t Write error: [%v]", err)
				c.Close()
				break
			}
		}

		n, err := c.Read(buf)
		if err != nil {
			log.Printf("relayClientHandler\t Read error: [%v]", err)
			c.Close()
			break
		}

		log.Printf("relayClientHandler\t Read: [%d]", n)
		nq <- buf
		log.Printf("relayClientHandler\t Sent to queue")

	}
	c.Close()
	log.Println("relayClientHandler\t terminated")
}

func relayServer(nq chan []byte) {
	retry := GlobalConfig.MaxRetries
	for retry > 0 {
		port := GetPort()
		addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)

		l, err := net.Listen("tcp", addr)
		if err == nil {
			defer l.Close()

			log.Printf("relayServer is running on: [%s]", addr)
			for {
				conn, err := l.Accept()
				CheckError(err)

				go relayServerHandler(conn, nq)
			}
			return
		}
		ReleasePort(port)
		retry--
	}
}

func relayServerHandler(c net.Conn, nq chan []byte) {

	log.Println("relayServerHandler spawned")

	buf := make([]byte, GlobalConfig.NetBufferSize)
	for {
		n, err := c.Read(buf)
		if err != nil {
			log.Printf("Read error: [%v]", err)
			c.Close()
			break
		}

		log.Printf("relayServerHandler\t Read: [%d]", n)
		nq <- buf
		log.Printf("relayServerHandler\t Sent to queue")

		log.Printf("Waiting for answer")
		select {
		case out := <-nq:
			n, err := c.Write(out)
			log.Printf("relayServerHandler\t Write: [%d]", n)
			if err != nil {
				log.Printf("relayServerHandler\t Write error: [%v]", err)
				c.Close()
				break
			}
		}

	}
	c.Close()
	log.Println("relayServerHandler terminated")
}

func GetPort() int {
	mutexPortsPool.Lock()
	defer mutexPortsPool.Unlock()

	counter := 65000
	for {
		pool.NextPort++

		if pool.ArPorts[pool.NextPort] == 0 {
			pool.ArPorts[pool.NextPort] = 1
			return pool.NextPort
		}

		if pool.NextPort > 65000 {
			pool.NextPort = 0
		}

		counter--
		if counter < 0 {
			break
		}
	}

	log.Println("Error, no ports available left")
	return -1
}

func ReleasePort(port int) {
	mutexPortsPool.Lock()
	defer mutexPortsPool.Unlock()

	pool.ArPorts[port] = 0
}
