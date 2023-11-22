package main

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
)

func main() {
	client := redis.NewClient(&redis.Options{})
	if err := http.ListenAndServe("localhost:8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i, err := client.Incr(r.Context(), "counter").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			_, _ = w.Write([]byte(fmt.Sprintf("%016x", i)))
		}
	})); err != nil {
		panic(err)
	}
}
