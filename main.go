package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net"
	"regexp"
)

var removePort = regexp.MustCompile(`^\[?(.*)\]?:[0-9]*$`)

func main() {
	sshConfig := ssh.ServerConfig{
		NoClientAuth: true,
	}

	privateBytes, err := ioutil.ReadFile("/etc/ssh/ssh_host_rsa_key")
	if err != nil {
		log.Fatal("Failed to load private key (id_rsa)\n")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key\n")
	}

	sshConfig.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:22")
	if err != nil {
		log.Fatalf("Failed to listen on port 22 (%s)\n", err)
	}

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept (%s)\n", err)
			continue
		}

		go func() {
			keys := make([]ssh.PublicKey, 0)
			sshConn, sshChannels, channelReqs, err := ssh.NewServerConn(tcpConn, &sshConfig)
			if err != nil {
				log.Printf("Failed to handshake (%s)\n", err)
				return
			}
			log.Printf("New SSH connection from %s (%s)\n", sshConn.RemoteAddr(), sshConn.ClientVersion())
			go ssh.DiscardRequests(channelReqs)

			for c := range sshChannels {
				channel := c
				go func() {
					if t := channel.ChannelType(); t != "session" {
						log.Printf("Rejecting channel type %s\n", t)
						err := channel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
						if err != nil {
							log.Printf("Failed to reject channel type %s (%s)\n", t, err)
							return
						}
					}

					conn, reqs, err := channel.Accept()
					if err != nil {
						log.Printf("Could not accept channel (%s)\n", err)
						return
					}

					go func() {
						for req := range reqs {
							switch req.Type {
							case "shell", "exec":
								go handleRequest(req, sshConn, keys, conn)
								err = req.Reply(true, nil)
								if err != nil {
									log.Printf("Could not reply (%s)\n", err)
								}
							default:
								log.Printf("Accepted type %s\n", req.Type)
								err = req.Reply(true, nil)
								if err != nil {
									log.Printf("Could not reply (%s)\n", err)
								}
							}
						}
					}()
				}()
			}
		}()
	}
}

func handleRequest(req *ssh.Request, sshConn *ssh.ServerConn, keys []ssh.PublicKey, conn ssh.Channel) {
	cmd := string(req.Payload)
	response := ""
	switch cmd {
	case "":
		response = removePort.ReplaceAllString(sshConn.RemoteAddr().String(), "$1")
	}
	if _, err := conn.Write([]byte(response)); err != nil {
		log.Printf("Could not write answer (%s)\n", err)
	}
	if err := conn.Close(); err != nil {
		log.Printf("Could not close (%s)\n", err)
	}
}
