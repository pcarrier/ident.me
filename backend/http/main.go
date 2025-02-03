package main

import (
	"context"
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

func trackRequest(ctx context.Context, rdb *redis.Client, ip string) error {
	now := time.Now().UTC()
	hourKey := fmt.Sprintf("r:h:%d", now.Unix()/3600)
	dayKey := fmt.Sprintf("r:d:%d", now.Unix()/86400)

	pipe := rdb.Pipeline()

	pipe.Incr(ctx, hourKey)
	pipe.Incr(ctx, dayKey)

	pipe.SAdd(ctx, "u:h:"+hourKey, ip)
	pipe.SAdd(ctx, "u:d:"+dayKey, ip)

	pipe.Expire(ctx, hourKey, 169*time.Hour)
	pipe.Expire(ctx, dayKey, 31*24*time.Hour)
	pipe.Expire(ctx, "u:h:"+hourKey, 169*time.Hour)
	pipe.Expire(ctx, "u:d:"+dayKey, 31*24*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
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
		clientIP := ip(r)
		if err := trackRequest(r.Context(), redis, clientIP); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Write([]byte(clientIP))
	})

	router.HandleFunc("/.json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		clientIP := ip(r)
		if err := trackRequest(r.Context(), redis, clientIP); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"address": "` + clientIP + `"}`))
	})

	router.HandleFunc("/.xml", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")
		clientIP := ip(r)
		if err := trackRequest(r.Context(), redis, clientIP); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Write([]byte(`<address>` + clientIP + `</address>`))
	})

	router.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")
		clientIP := ip(r)
		if err := trackRequest(r.Context(), redis, clientIP); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		pip := net.ParseIP(clientIP)
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
		resp, err := json.Marshal(toJSON(clientIP, city, asn))
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

	router.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")

		now := time.Now().UTC()
		prevHourKey := fmt.Sprintf("r:h:%d", now.Unix()/3600-1)
		prevDayKey := fmt.Sprintf("r:d:%d", now.Unix()/86400-1)
		hourKey := fmt.Sprintf("r:h:%d", now.Unix()/3600)
		dayKey := fmt.Sprintf("r:d:%d", now.Unix()/86400)

		pipe := redis.Pipeline()
		reqsPrevHour := pipe.Get(r.Context(), prevHourKey)
		reqsThisHour := pipe.Get(r.Context(), hourKey)
		reqsPrevDay := pipe.Get(r.Context(), prevDayKey)
		reqsToday := pipe.Get(r.Context(), dayKey)
		ipsThisHour := pipe.SCard(r.Context(), "u:h:"+hourKey)
		ipsToday := pipe.SCard(r.Context(), "u:d:"+dayKey)

		_, err := pipe.Exec(r.Context())
		if err != nil {
			fmt.Printf("Error getting stats: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Helper function to safely get value or return 0
		getVal := func(result *redis.StringCmd) int64 {
			val, err := result.Int64()
			if err == redis.Nil {
				return 0
			}
			return val
		}

		stats := map[string]interface{}{
			"reqsPrevHour": getVal(reqsPrevHour),
			"reqsThisHour": getVal(reqsThisHour),
			"reqsPrevDay":  getVal(reqsPrevDay),
			"reqsToday":    getVal(reqsToday),
			"ipsThisHour":  getVal(ipsThisHour),
			"ipsToday":     getVal(ipsToday),
		}
		json.NewEncoder(w).Encode(stats)
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
