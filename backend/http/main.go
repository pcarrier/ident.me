package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/openrdap/rdap"
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
		"a.ident.me", "any.ident.me", "4.ident.me", "6.ident.me", "ident.me", "ip4.ident.me", "ip6.ident.me", "ipv4.ident.me", "ipv6.ident.me", "v4.ident.me", "v6.ident.me",
		"a.tnedi.me", "any.tnedi.me", "4.tnedi.me", "6.tnedi.me", "tnedi.me", "ip4.tnedi.me", "ip6.tnedi.me", "ipv4.tnedi.me", "ipv6.tnedi.me", "v4.tnedi.me", "v6.tnedi.me",
	}
	userAgents = make([]string, 8192)
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

func trackRequest(ctx context.Context, rdb *redis.Client, ip string, userAgent string) error {
	if userAgent == "" {
		userAgent = "unknown"
	}
	if decoded, err := url.QueryUnescape(userAgent); err == nil {
		userAgent = decoded
	}
	parts := strings.FieldsFunc(userAgent, func(r rune) bool {
		return r == '/' || r == ' ' || r == ';'
	})
	userAgent = parts[0]
	idx := rand.Intn(len(userAgents))
	userAgents[idx] = userAgent

	now := time.Now().UTC()
	hour := strconv.FormatInt(now.Unix()/3600, 10)
	day := strconv.FormatInt(now.Unix()/86400, 10)

	pipe := rdb.Pipeline()

	pipe.Incr(ctx, "h:"+hour)
	pipe.Incr(ctx, "d:"+day)

	pipe.PFAdd(ctx, "ph:"+hour, ip)
	pipe.PFAdd(ctx, "pd:"+day, ip)

	pipe.Expire(ctx, "h:"+hour, 169*time.Hour)
	pipe.Expire(ctx, "d:"+day, 31*24*time.Hour)
	pipe.Expire(ctx, "ph:"+hour, 169*time.Hour)
	pipe.Expire(ctx, "pd:"+day, 31*24*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

func getRedisInt64(result redis.Cmder) int64 {
	switch v := result.(type) {
	case *redis.StringCmd:
		val, err := v.Int64()
		if err == redis.Nil {
			return 0
		}
		if err != nil {
			return 0
		}
		return val
	case *redis.IntCmd:
		return v.Val()
	default:
		return 0
	}
}

type RDAPHTTP struct {
	URL    string      `json:"url"`
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
}

func main() {
	red := redis.NewClient(&redis.Options{})
	rdapc := rdap.Client{}
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
		accept := r.Header.Get("Accept")
		if strings.Count(r.Host, ".") == 1 && (accept == "text/html" || strings.HasPrefix(accept, "text/html,")) {
			w.Header().Set("Location", "https://www."+r.Host)
			w.WriteHeader(http.StatusSeeOther)
		}
		clientIP := ip(r)
		if err := trackRequest(r.Context(), red, clientIP, r.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Write([]byte(clientIP))
	})

	router.HandleFunc("/.json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		clientIP := ip(r)
		if err := trackRequest(r.Context(), red, clientIP, r.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"address": "` + clientIP + `"}`))
	})

	router.HandleFunc("/.xml", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/xml")
		clientIP := ip(r)
		if err := trackRequest(r.Context(), red, clientIP, r.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		w.Write([]byte(`<address>` + clientIP + `</address>`))
	})

	router.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")
		clientIP := ip(r)
		if err := trackRequest(r.Context(), red, clientIP, r.UserAgent()); err != nil {
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
		i, err := red.Incr(r.Context(), "counter").Result()
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
		pipe := red.Pipeline()

		// Get last 24 hours of hourly stats
		hourlyStats := make([]int64, 24)
		hourlyUniqueStats := make([]int64, 24)
		for i := 0; i < 24; i++ {
			hour := strconv.FormatInt(now.Unix()/3600-int64(i), 10)
			pipe.Get(r.Context(), "h:"+hour)
			pipe.PFCount(r.Context(), "ph:"+hour)
		}

		// Get last 30 days of daily stats
		dailyStats := make([]int64, 30)
		dailyUniqueStats := make([]int64, 30)
		for i := 0; i < 30; i++ {
			day := strconv.FormatInt(now.Unix()/86400-int64(i), 10)
			pipe.Get(r.Context(), "d:"+day)
			pipe.PFCount(r.Context(), "pd:"+day)
		}

		results, err := pipe.Exec(r.Context())
		if err != nil && err != redis.Nil {
			fmt.Printf("Error getting stats: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resultIdx := 0
		for i := 0; i < 24; i++ {
			hourlyStats[i] = getRedisInt64(results[resultIdx])
			resultIdx++
			hourlyUniqueStats[i] = getRedisInt64(results[resultIdx])
			resultIdx++
		}
		for i := 0; i < 30; i++ {
			dailyStats[i] = getRedisInt64(results[resultIdx])
			resultIdx++
			dailyUniqueStats[i] = getRedisInt64(results[resultIdx])
			resultIdx++
		}

		uaCount := make(map[string]int)
		for _, ua := range userAgents {
			if ua != "" {
				uaCount[ua]++
			}
		}

		stats := map[string]interface{}{
			"hourly": map[string]interface{}{
				"reqs": hourlyStats,
				"ips":  hourlyUniqueStats,
			},
			"daily": map[string]interface{}{
				"reqs": dailyStats,
				"ips":  dailyUniqueStats,
			},
			"ua": uaCount,
		}
		json.NewEncoder(w).Encode(stats)
	})

	router.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "text/plain")
		r.Header.Write(w)
	})

	router.HandleFunc("/rdap/", func(w http.ResponseWriter, r *http.Request) {
		headers(w)
		w.Header().Set("Content-Type", "application/json")

		target := strings.TrimPrefix(r.URL.Path, "/rdap/")
		if target == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		req := rdap.NewAutoRequest(target)
		if req == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		req = req.WithContext(r.Context())

		resp, err := rdapc.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(err.Error()))
			return
		}

		// Write all HTTP response resps to the response writer
		resps := make([]RDAPHTTP, len(resp.HTTP))
		for i, h := range resp.HTTP {
			var body interface{}
			if err := json.Unmarshal(h.Body, &body); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			resps[i] = RDAPHTTP{
				URL:    h.URL,
				Status: h.Response.StatusCode,
				Body:   body,
			}
		}
		w.WriteHeader(http.StatusOK)
		bytes, err := json.Marshal(resps)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(bytes)
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
