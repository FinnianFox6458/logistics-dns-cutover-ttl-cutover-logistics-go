package main

import (
	"log"
	"net/http"
	"os"

	cutover "github.com/example/logistics-dns-cutover"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}
	service := cutover.Service{DNS: cutover.NewClient(key)}
	log.Printf("logistics cutover service listening on %s", address)
	log.Fatal(http.ListenAndServe(address, cutover.Handler(service)))
}
