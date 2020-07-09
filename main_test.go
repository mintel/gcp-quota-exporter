package main

import (
	"os"
	"testing"
)

func TestScrape(t *testing.T) {

	// TestSuccessfulConnection
	exporter, _ := NewExporter(os.Getenv("GCP_PROJECT"))
	up, _, _ := exporter.scrape()
	if up == 0 {
		t.Errorf("TestSuccessfulConnection: up=%v, expected=1", up)
	}

	// TestFailedConnection
	// Set the project name to "503" since the Google Compute API will append this to the end of the BasePath
	exporter, _ = NewExporter("503")
	exporter.service.BasePath = "https://httpbin.org/status/"
	up, _, _ = exporter.scrape()
	if up != 0 {
		t.Errorf("TestFailedConnection: up=%v, expected=0", up)
	}
}
