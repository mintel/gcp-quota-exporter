package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"unsafe"
)

const ptrSize = unsafe.Sizeof(new(int))

func TestNewExporter(t *testing.T) {
	// Set the project name to "503" since the Google Compute API will append this to the end of the BasePath
	exporter, _ := NewExporter("credentials.json", "503")
	exporter.service.BasePath = "https://httpbin.org/status/"
	err := exporter.getProjectQuotas(make(chan<- prometheus.Metric))
}