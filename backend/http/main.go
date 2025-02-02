package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/acme/autocert"
)

type JSON struct {
	IP        string  `json:"ip,omitempty"`
	ASO       string  `json:"aso,omitempty"`
	ASN       uint    `json:"asn,omitempty"`
	Continent string  `json:"continent,omitempty"`
	CC        string  `json:"cc,omitempty"`
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Postal    string  `json:"postal,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	TZ        string  `json:"tz,omitempty"`
}

func toJSON(ip string, city *geoip2.City, asn *geoip2.ASN) JSON {
	return JSON{
		IP:        ip,
		ASO:       asn.AutonomousSystemOrganization,
		ASN:       asn.AutonomousSystemNumber,
		Continent: city.Continent.Code,
		CC:        city.Country.IsoCode,
		Country:   city.Country.Names["en"],
		City:      city.City.Names["en"],
		Postal:    city.Postal.Code,
		Latitude:  city.Location.Latitude,
		Longitude: city.Location.Longitude,
		TZ:        city.Location.TimeZone,
	}
}

var (
	domains = []string{
		"any.ident.me", "4.ident.me", "6.ident.me", "ident.me", "ip4.ident.me", "ip6.ident.me", "ipv4.ident.me", "ipv6.ident.me", "v4.ident.me", "v6.ident.me",
		"any.tnedi.me", "4.tnedi.me", "6.tnedi.me", "tnedi.me", "ip4.tnedi.me", "ip6.tnedi.me", "ipv4.tnedi.me", "ipv6.tnedi.me", "v4.tnedi.me", "v6.tnedi.me",
	}
)

func ip(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func headers(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
}

func main() {
	redis := redis.NewClient(&redis.Options{})
	city, err := geoip2.Open("/usr/share/GeoIP/GeoLite2-City.mmdb")
	if err != nil {
		panic(err)
	}
	asn, err := geoip2.Open("/usr/share/GeoIP/GeoLite2-ASN.mmdb")
	if err != nil {
		panic(err)
	}

	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache("/certs"),
	}

	router := http.NewServeMux()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Write([]byte(ip(r)))
	})

	router.HandleFunc("/.json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"address": "` + ip(r) + `"}`))
	})

	router.HandleFunc("/.xml", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<address>` + ip(r) + `</address>`))
	})

	router.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		ip := ip(r)
		pip := net.ParseIP(ip)
		city, err := city.City(pip)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		asn, err := asn.ASN(pip)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp, err := json.Marshal(toJSON(ip, city, asn))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	})

	router.HandleFunc("/n", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		i, err := redis.Incr(r.Context(), "counter").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			_, _ = w.Write([]byte(fmt.Sprintf("%016x", i)))
		}
	})

	go func() {
		server80 := &http.Server{
			Addr:         ":80",
			Handler:      certManager.HTTPHandler(router),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  65 * time.Second,
		}

		if err := server80.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	serverTLS := &http.Server{
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  65 * time.Second,
	}

	if err := serverTLS.Serve(certManager.Listener()); err != nil {
		panic(err)
	}
}
