package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"
)

var (
	removePort = regexp.MustCompile(`^\[?([^\]]*)\]?:\d*$`)
)

func serve(conn net.Conn) {
	ip := removePort.ReplaceAllString(conn.RemoteAddr().String(), "$1")
	log.Printf("Resolved %s", ip)
	if _, err := conn.Write([]byte(ip)); err != nil {
		log.Printf("Failed to respond to %s (%s)", ip, err)
	}
	if err := conn.Close(); err != nil {
		log.Printf("Failed to close connection (%s)", err)
	}
}

func main() {
	go func() {
		server, err := net.Listen("tcp", ":23")
		if err != nil {
			log.Fatalln(err)
		}
		defer server.Close()

		for {
			conn, err := server.Accept()
			if err != nil {
				log.Printf("Failed to accept connection (%s)", err)
				continue
			}

			go serve(conn)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
