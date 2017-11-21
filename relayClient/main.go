// relayClient, for built simplification, this file contains all method and not being split into packages.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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

	input := bufio.NewReader(conn)
	output := bufio.NewWriter(conn)

	for {
		// Reading response from the server, either here or in separate case
		out := make([]byte, 0, 256)
		tmp := make([]byte, 1)
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
				break
			}

			out = append(out, tmp[:r]...)
			// log.Println("SIZE: ", len(out), "Payload:", string(out))

			bf := input.Buffered()
			log.Printf("Buffered: %d, read: %d", bf, r)

			if bf <= 0 {
				break
			}
		}

		w, e := output.Write(out)
		if e != nil || w <= 0 {
			log.Printf("network write error: [%v]", e)
			return
		}
		log.Printf("Client: [%d] bytes written, payload: [%s]", w, string(out))
		output.Flush()
	}
}

func main() {

	err, c := Init()
	CheckError(err)

	StartServer(c)
}
