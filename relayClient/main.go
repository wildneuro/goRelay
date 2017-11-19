// relayClient, for built simplification, this file contains all method and not being split into packages.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

type Config struct {
	RelayHost string
	RelayPort int
}

var (
	relayHost = flag.String("host", "127.0.0.1", "Relay host")
	relayPort = flag.Int("port", 10000, "Relay port")
)

func Init() (error, Config) {
	c := Config{
		RelayHost: *relayHost,
		RelayPort: *relayPort,
	}

	return nil, c
}

func CheckError(err error) {
	if err == nil {
		return
	}

	log.Printf("Error occuried: [%v]\n", err)
	os.Exit(1)
}

func StartServer(c Config) {
	addr := fmt.Sprintf("%s:%d", c.RelayHost, c.RelayPort)

	conn, err := net.Dial("tcp", addr)
	CheckError(err)

	defer conn.Close()

	log.Println("Client is running...")
	buf := make([]byte, 1024)
	for {
		log.Println("Reading...")
		r, errR := conn.Read(buf)
		CheckError(errR)
		log.Printf("Client: [%d] bytes read", r)

		log.Println("Writing...")
		w, errW := conn.Write(buf)
		CheckError(errW)
		log.Printf("Client: [%d] bytes written", w)
	}
}

func main() {

	err, c := Init()
	CheckError(err)

	StartServer(c)
}
