package main

import (
	"log"
	"net"
	"regexp"
)

var removePort = regexp.MustCompile(`^\[?([^\]]*)\]?:[0-9]*$`)

func main() {
	server, err := net.Listen("tcp", ":23")
	if err != nil {
		log.Fatalln(err)
	}
	defer server.Close()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Failed to accept connection (%s)\n", err)
			continue
		}

		go func() {
			defer func() {
				if err := conn.Close(); err != nil {
					log.Printf("Failed to close connection (%s)\n", err)
				}
			}()
			if _, err := conn.Write([]byte(removePort.ReplaceAllString(conn.RemoteAddr().String(), "$1"))); err != nil {
				log.Printf("Failed to close connection (%s)\n", err)
			}
		}()
	}
}
