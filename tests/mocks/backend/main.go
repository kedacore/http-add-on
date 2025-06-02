package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	fmt.Printf("[%s] Mock backend started\n", time.Now().Format(time.RFC850))

	http.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "Live")
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "Ready")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		delayDuration, err := time.ParseDuration(r.URL.Query().Get("delay"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}

		time.Sleep(delayDuration)

		fmt.Fprintf(w, "OK: delayed for %v\n", delayDuration)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
