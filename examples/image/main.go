package main

import (
	"fmt"
	"log"
	"net/http"
)

const (
	PORT = 8080
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello 🖖")
		fmt.Println("Hello 🖖")
	})
	fmt.Printf("Running server on port: %d\n", PORT)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil))
}
