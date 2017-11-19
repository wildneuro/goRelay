// relayServer, for built simplification, this file contains all method and not being split into packages.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
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

type Ipc struct {
	DataQueue       chan []byte // Data channel
	ServerCtrlQueue chan int    // Control channel
	ClientCtrlQueue chan int    // Control channel
}

const (
	CtrlStopAccept = iota
	CtrlStopClient
)

var (
	GlobalConfig   = Config{}
	mutexPortsPool = &sync.Mutex{}
	pool           = Pool{}

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

	pool = Pool{
		ArPorts:  make([]int, 65535),
		NextPort: 20000, // Starts from port: 20000
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

// Start the main server
func startServer() {

	defer func() {
		if recover() != nil {
			log.Println("Houston, we have a panic, recovering")
		}
	}()

	addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, GlobalConfig.ListenPort)

	l, err := net.Listen("tcp", addr)
	CheckError(err)

	defer l.Close()

	log.Println("Server is running...")

	for {
		conn, err := l.Accept()
		CheckError(err)

		remoteAddr := conn.RemoteAddr()
		log.Println("Relay Server connected, from addr: ", remoteAddr.String())
		ipc := Ipc{
			DataQueue:       make(chan []byte),
			ServerCtrlQueue: make(chan int),
			ClientCtrlQueue: make(chan int),
		}

		go ClientHandler(conn, ipc)
	}
}

// Using TCPListener to have SetDeadLine feature
func relayServer(ipc Ipc, l *net.TCPListener) {

	for {
		conn, err := l.Accept()

		if err != nil {

			select {
			case ctrl := <-ipc.ServerCtrlQueue:
				log.Printf("ServerCtrlQueue: [%d]", ctrl)
				if ctrl == CtrlStopAccept {
					log.Println("Control: quit")
					return
				}
			default:
			}

			// Accept is always blocking, workarounds are:
			// 1. Close the Listener
			// 2. Set dead lines
			l.SetDeadline(time.Now().Add(1 * time.Second))

			continue
		}

		remoteAddr := conn.RemoteAddr()
		log.Println("Relay Client connected, from addr: ", remoteAddr.String())

		go relayServerHandler(conn, ipc)
	}
}

// Implements ClientHandler
func ClientHandler(c net.Conn, ipc Ipc) {

	var (
		l         *net.TCPListener
		errListen error
	)
	// log.Println("ClientHandler\t spawned")

	retry := GlobalConfig.MaxRetries // N-attempts to get the next free port
	for retry > 0 {
		port := GetPort()
		addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, port)

		lAddr, lAddrErr := net.ResolveTCPAddr("tcp", addr)
		if lAddrErr != nil {
			log.Panic("Cannot resolve TCP addr: [%s]", addr)
		}

		l, errListen = net.ListenTCP("tcp", lAddr)
		defer l.Close()

		l.SetDeadline(time.Now().Add(5 * time.Second))

		if errListen != nil {
			l.Close()

			ReleasePort(port)
			retry--

			time.Sleep(100 * time.Millisecond) // Wait before pick the next port

			continue
		}

		log.Printf("Relay Server is running on: [%s]", addr)

		break
	}

	go relayServer(ipc, l)

	buf := make([]byte, GlobalConfig.NetBufferSize)
	for {
		select {
		case out := <-ipc.DataQueue:
			w, err := c.Write(out)
			if err != nil {
				log.Printf("ClientHandler\t Write error: [%v]", err)
				goto terminated
			}
			if w <= 0 {
				log.Printf("ClientHandler\t Write error, 0 bytes sent")
				goto terminated
			}

			r, err := c.Read(buf)
			if err != nil {
				log.Printf("ClientHandler\t Read error: [%v]", err)
				goto terminated
			}
			if r <= 0 {
				log.Printf("ClientHandler\t Read error, 0 bytes read")
				goto terminated
			}

			ipc.DataQueue <- buf
		}

	}

	// Rare when labels are ok:
terminated:

	ipc.ServerCtrlQueue <- CtrlStopAccept // Breaking from Accept loop
	ipc.ClientCtrlQueue <- CtrlStopClient // Sending 'disconnect' to the relayClient

	l.Close() // Closing Listener
	c.Close() // Closing the connection

	// log.Println("ClientHandler\t terminated")

	return
}

// Implements a handler for relayServer
func relayServerHandler(c net.Conn, ipc Ipc) {

	// log.Println("relayServerHandler spawned")

	buf := make([]byte, GlobalConfig.NetBufferSize)
	for {
		n, err := c.Read(buf)
		if err != nil {
			log.Printf("relayServerHandler\t Read error: [%v]", err)
			goto terminated
		}
		if n <= 0 {
			log.Printf("relayServerHandler\t Write error, 0 bytes read")
			goto terminated
		}

		ipc.DataQueue <- buf

		select {
		case out := <-ipc.DataQueue:
			n, err := c.Write(out)
			if err != nil {
				log.Printf("relayServerHandler\t Write error: [%v]", err)
				goto terminated
			}
			if n <= 0 {
				log.Printf("relayServerHandler\t Write error, 0 bytes sent")
				goto terminated
			}
		case ctrl := <-ipc.ClientCtrlQueue:
			log.Printf("ClientCtrlQueue: [%d]", ctrl)
			if ctrl == CtrlStopClient {
				log.Println("Control: stop client")
				goto terminated
			}
		}

	}

terminated:
	// log.Println("relayServerHandler terminated")
	close(ipc.ClientCtrlQueue)
	close(ipc.ServerCtrlQueue)
	c.Close()
	return
}

// GetPort helps to get the next available port from the pool
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

// ReleasePort makes the port release to be available
func ReleasePort(port int) {
	mutexPortsPool.Lock()
	defer mutexPortsPool.Unlock()

	pool.ArPorts[port] = 0
}

// Main function
func main() {
	Init()
	startServer()
}
