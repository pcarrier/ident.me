package main

import (
	"github.com/miekg/dns"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func handle(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	name := r.Question[0].Name

	var a net.IP

	if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		a = ip.IP
	} else if ip, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		a = ip.IP
	}

	if a.To4() != nil {
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   a,
		})
	} else {
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: a,
		})
	}
	w.WriteMsg(m)
}

func main() {
	dns.HandleFunc(".", handle)
	go func() {
		if err := (&dns.Server{Addr: ":53", Net: "udp", ReusePort: true}).ListenAndServe(); err != nil {
			log.Fatalf("Failed to listen on UDP (%s)", err)
		}
	}()
	go func() {
		if err := (&dns.Server{Addr: ":53", Net: "tcp", ReusePort: true}).ListenAndServe(); err != nil {
			log.Fatalf("Failed to listen on TCP (%s)", err)
		}
	}()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
