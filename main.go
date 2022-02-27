package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"syscall"
)

var removePort = regexp.MustCompile(`^\[?([^\]]*)\]?:[0-9]*$`)

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
				log.Printf("Failed to accept connection (%s)\n", err)
				continue
			}

			go func(conn net.Conn) {
				if _, err := conn.Write([]byte(removePort.ReplaceAllString(conn.RemoteAddr().String(), "$1"))); err != nil {
					log.Printf("Failed to write (%s)\n", err)
				}
				if err := conn.Close(); err != nil {
					log.Printf("Failed to close connection (%s)\n", err)
				}
			}(conn)
		}
	}()

	go func() {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalln(err)
		}

		cert, err := tls.LoadX509KeyPair("/etc/letsencrypt/live/"+hostname+".me/fullchain.pem", "/etc/letsencrypt/live/"+hostname+".me/privkey.pem")
		if err != nil {
			log.Fatalln(err)
		}

		config := &tls.Config{Certificates: []tls.Certificate{cert}}

		server, err := tls.Listen("tcp", ":992", config)
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

			go func(conn net.Conn) {
				if _, err := conn.Write([]byte(removePort.ReplaceAllString(conn.RemoteAddr().String(), "$1"))); err != nil {
					log.Printf("Failed to write (%s)\n", err)
				}
				if err := conn.Close(); err != nil {
					log.Printf("Failed to close connection (%s)\n", err)
				}
			}(conn)
		}
	}()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
