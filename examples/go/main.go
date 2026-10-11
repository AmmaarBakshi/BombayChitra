// Lists the metro and suburban railway lines with their colours and lengths.
//
//	go run main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type line struct {
	Label   string  `json:"label"`
	Network string  `json:"network"`
	Mode    string  `json:"mode"`
	Colour  string  `json:"colour"`
	LengthM float64 `json:"length_m"`
	Routes  []struct {
		StopCount int `json:"stop_count"`
	} `json:"routes"`
}

func main() {
	base := os.Getenv("BOMBAYCHITRA_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(base + "/api/v1/transit/lines")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status %s", resp.Status)
	}
	var body struct {
		Items []line `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		log.Fatal(err)
	}
	for _, l := range body.Items {
		stops := 0
		for _, r := range l.Routes {
			stops = max(stops, r.StopCount)
		}
		fmt.Printf("%-26s %-24s %-8s %6.1f km  %3d stations\n", l.Network, l.Label, l.Colour, l.LengthM/1000, stops)
	}
}
