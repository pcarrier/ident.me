package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/openrdap/rdap"
	"github.com/oschwald/geoip2-golang"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/acme"
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

func ip(ctx *fasthttp.RequestCtx) string {
	return ctx.RemoteIP().String()
}

func headers(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
}

func trackRequest(ctx context.Context, rdb *redis.Client, ip string, userAgent []byte) error {
	ua := string(userAgent)
	if ua == "" {
		ua = "unknown"
	}
	if decoded, err := url.QueryUnescape(ua); err == nil {
		ua = decoded
	}
	parts := strings.FieldsFunc(ua, func(r rune) bool {
		return r == '/' || r == ' ' || r == ';'
	})
	ua = parts[0]
	idx := rand.Intn(len(userAgents))
	userAgents[idx] = ua

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

	r := router.New()

	r.GET("/", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		accept := string(ctx.Request.Header.Peek("Accept"))
		if strings.Count(string(ctx.Host()), ".") == 1 && (accept == "text/html" || strings.HasPrefix(accept, "text/html,")) {
			ctx.Response.Header.Set("Location", "https://www."+string(ctx.Host()))
			ctx.SetStatusCode(fasthttp.StatusSeeOther)
			return
		}
		clientIP := ip(ctx)
		if err := trackRequest(ctx, red, clientIP, ctx.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		ctx.SetBodyString(clientIP)
	})

	r.GET("/.json", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		clientIP := ip(ctx)
		if err := trackRequest(ctx, red, clientIP, ctx.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		ctx.SetContentType("application/json")
		ctx.SetBodyString(fmt.Sprintf(`{"address": "%s"}`, clientIP))
	})

	r.GET("/.xml", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		ctx.Response.Header.Set("Content-Type", "application/xml")
		clientIP := ip(ctx)
		if err := trackRequest(ctx, red, clientIP, ctx.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		ctx.SetBodyString(fmt.Sprintf(`<address>%s</address>`, clientIP))
	})

	r.GET("/json", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		ctx.SetContentType("application/json")
		clientIP := ip(ctx)
		if err := trackRequest(ctx, red, clientIP, ctx.UserAgent()); err != nil {
			fmt.Printf("Error tracking request: %v\n", err)
		}
		pip := net.ParseIP(clientIP)
		cityResult, err := city.City(pip)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			return
		}
		asnResult, err := asn.ASN(pip)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			return
		}
		resp, err := json.Marshal(toJSON(clientIP, cityResult, asnResult))
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			return
		}
		ctx.SetBody(resp)
	})

	r.GET("/n", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		i, err := red.Incr(ctx, "counter").Result()
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		} else {
			_, _ = ctx.WriteString(fmt.Sprintf("%016x", i))
		}
	})

	r.GET("/stats", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		ctx.Response.Header.Set("Content-Type", "application/json")

		now := time.Now().UTC()
		pipe := red.Pipeline()

		// Get last 24 hours of hourly stats
		hourlyStats := make([]int64, 24)
		hourlyUniqueStats := make([]int64, 24)
		for i := 0; i < 24; i++ {
			hourKey := fmt.Sprintf("r:h:%d", now.Unix()/3600-int64(i))
			pipe.Get(ctx, hourKey)
			pipe.SCard(ctx, "u:h:"+hourKey)
		}

		// Get last 30 days of daily stats
		dailyStats := make([]int64, 30)
		dailyUniqueStats := make([]int64, 30)
		for i := 0; i < 30; i++ {
			dayKey := fmt.Sprintf("r:d:%d", now.Unix()/86400-int64(i))
			pipe.Get(ctx, dayKey)
			pipe.SCard(ctx, "u:d:"+dayKey)
		}

		results, err := pipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			fmt.Printf("Error getting stats: %v\n", err)
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
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
		json.NewEncoder(ctx).Encode(stats)
	})

	r.GET("/headers", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		ctx.Response.Header.Set("Content-Type", "text/plain")
		buf := bufio.NewWriter(ctx.Response.BodyWriter())
		ctx.Request.Header.Write(buf)
		_ = buf.Flush()
	})

	r.GET("/rdap/", func(ctx *fasthttp.RequestCtx) {
		headers(ctx)
		ctx.Response.Header.Set("Content-Type", "application/json")

		target := strings.TrimPrefix(string(ctx.Path()), "/rdap/")
		if target == "" {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			return
		}

		req := rdap.NewAutoRequest(target)
		if req == nil {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			return
		}

		req = req.WithContext(ctx)

		resp, err := rdapc.Do(req)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			ctx.SetBodyString(err.Error())
			return
		}

		// Write all HTTP response resps to the response writer
		resps := make([]RDAPHTTP, len(resp.HTTP))
		for i, h := range resp.HTTP {
			var body interface{}
			if err := json.Unmarshal(h.Body, &body); err != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBodyString(err.Error())
				return
			}
			resps[i] = RDAPHTTP{
				URL:    h.URL,
				Status: h.Response.StatusCode,
				Body:   body,
			}
		}
		ctx.SetStatusCode(fasthttp.StatusOK)
		bytes, err := json.Marshal(resps)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBodyString(err.Error())
			return
		}
		ctx.SetBody(bytes)
	})

	go func() {
		if err := fasthttp.ListenAndServe(":80", r.Handler); err != nil {
			panic(err)
		}
	}()

	server := &fasthttp.Server{
		Handler: r.Handler,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			NextProtos:     []string{"http/1.1", acme.ALPNProto},
		},
	}

	if err := server.ListenAndServeTLS(":443", "", ""); err != nil {
		panic(err)
	}
}
