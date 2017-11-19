package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

type Config struct {
	RelayHost string
	RelayPort int
}

func Init() (error, Config) {
	c := Config{
		RelayHost: "127.0.0.1",
		RelayPort: 10000,
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

func main() {

	err, c := Init()
	CheckError(err)

	StartServer(c)
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
