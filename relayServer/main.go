// relayServer, for built simplification, this file contains all method and not being split into packages.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"strconv"
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
		NetBufferSize: 32,
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

	log.Println("relayServer is running...\n")

	for {
		connServer, err := l.Accept()
		CheckError(err)

		remoteAddr := connServer.RemoteAddr()
		log.Println("relay connected, from addr: ", remoteAddr.String())

		go clientServer(connServer)
	}
}

func clientServer(connServer net.Conn) {

	port := GetNextPort(20000)
	addr := fmt.Sprintf("%s:%d", GlobalConfig.ListenHost, port)
	grId := getGID()

	l, err := net.Listen("tcp", addr)
	CheckError(err)

	defer l.Close()

	log.Println("[", grId, "] \t clientServer is running on port:", port, "\n")

	for {
		connClient, err := l.Accept()
		CheckError(err)

		remoteAddr := connClient.RemoteAddr()
		log.Println("client connected, from addr:", remoteAddr.String())

		chanToServer := make(chan []byte)
		chanToClient := make(chan []byte)
		go clientHandler(connClient, connServer, chanToServer, chanToClient)
	}

	connServer.Close()
}

// Handles each Client To Server connection
func clientHandler(connClient, connServer net.Conn, chanToServer, chanToClient chan []byte) {

	wg := sync.WaitGroup{}
	clientAddr := connClient.RemoteAddr().String()

	log.Println("new clientHandler, for addr:", clientAddr)
	log.Println("chanToServer", chanToServer, "; chanToClient", chanToClient)

	wg.Add(3)
	go routeAll(connServer, chanToServer, chanToClient, &wg)
	go routeServerToClient(chanToClient, connClient, &wg)
	go routeClientToServer(connClient, chanToServer, &wg)

	wg.Wait()

	connClient.Close()

	return
}

// Helpers

// consumes data from Clients Channel and transmit them to the Server's connection
func routeAll(connServer net.Conn, chanToServer, chanToClient chan []byte, wg *sync.WaitGroup) {

	defer wg.Done()

	serverAddr := connServer.RemoteAddr().String()
	grId := getGID()

	input := bufio.NewReaderSize(connServer, GlobalConfig.NetBufferSize)
	output := bufio.NewWriter(connServer)

	log.Printf("[%d] \t Starting routeAll loop, for addr: [%s]", grId, serverAddr)
	for {
		select {
		case in := <-chanToServer:
			if w, e := output.Write(in); e != nil {
				log.Printf("network write error: [%v]", e)
				break
			} else {
				output.Flush()
				log.Printf("[%d] \t chanToServer: [%d], connServer.Written: [%d], payload: [%s]", grId, len(in), w, string(in))
			}

			// Reading response from the server, either here or in separate case
			out := make([]byte, 0, 256)
			tmp := make([]byte, GlobalConfig.NetBufferSize)
			for {
				r, e := input.Read(tmp)

				if e != nil {
					if e == io.EOF {
						break
					}
					log.Printf("network read error: [%v]", e)
					return
				}
				if r <= 0 {
					log.Printf("[%d] \t ServerToClient Transmission is done, sent: %d", grId, r)
					break
				}

				out = append(out, tmp[:r]...)
				bf := input.Buffered()

				if bf <= 0 {
					break
				}
			}
			log.Printf("[%d] \t chanToClient: [%d], payload: [%s]", grId, len(out), string(out))
			chanToClient <- out
		}
	}
	log.Printf("[%d] \t Stopping routeAll loop", grId)
}

// routes data from client's connection, to the originated chanToClient
func routeClientToServer(connClient net.Conn, chanToServer chan []byte, wg *sync.WaitGroup) {

	defer wg.Done()

	clientAddr := connClient.RemoteAddr().String()
	grId := getGID()

	input := bufio.NewReader(connClient)

	log.Printf("[%d] \t Starting routeClientToServer loop, for addr: [%s]", grId, clientAddr)
	for {
		out := make([]byte, 0, 256)
		tmp := make([]byte, GlobalConfig.NetBufferSize)
		for {
			r, e := input.Read(tmp)
			if e != nil {
				if e == io.EOF {
					break
				}
				log.Printf("network read error: [%v]", e)
				return
			}
			if r <= 0 {
				log.Printf("[%d] \t %s \t ClientToServer Transmission is done, sent: %d", r)
				break
			}
			log.Printf("[%d] \t %s \t Read bytes: %d", grId, clientAddr, r)

			out = append(out, tmp[:r]...)
			bf := input.Buffered()

			if bf <= 0 {
				break
			}
		}
		chanToServer <- out
		log.Printf("[%d] \t %s \t Data read and sent to channel: %d bytes \t Payload: [%s]", grId, clientAddr, len(out), string(out))
		// chanToServer <- out
	}

	log.Printf("[%d] \t Stopping routeClientToServer loop", grId)
}

// routes data from originated chanToServer, to the client's connection
func routeServerToClient(chanToClient chan []byte, connClient net.Conn, wg *sync.WaitGroup) {

	defer wg.Done()

	clientAddr := connClient.RemoteAddr().String()
	grId := getGID()

	log.Printf("[%d] \t Starting routeServerToClient loop, for addr: [%s]", grId, clientAddr)

	for {
		select {
		case in := <-chanToClient:
			w, e := connClient.Write(in)
			if e != nil {
				log.Printf("Error while Writing to Client, %v", e)
				break
			} else {
				log.Printf("[%d] \t %s \t Data sent back to the client : %d", grId, clientAddr, w)
			}
		}
	}
	log.Printf("[%d] \t Stopping routeServerToClient loop", grId)
}

// Get Next port from the pool
func GetNextPort(port int) int {
	if port > 65000 {
		port = 0
	}
	return port + 1
}

// Get routine Id
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

// Main function
func main() {
	Init()
	relayServer()
}
