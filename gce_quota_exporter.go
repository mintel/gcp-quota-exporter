package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "gce_quota"
)

var (
	limitDesc = prometheus.NewDesc("gce_quota_limit", "quota limits for GCE components", []string{"project", "region", "metric"}, nil)
	usageDesc = prometheus.NewDesc("gce_quota_usage", "quota usage for GCE components", []string{"project", "region", "metric"}, nil)
)

// Exporter collects quota stats from the Google Compute API and exports them using the Prometheus metrics package.
type Exporter struct {
	service *compute.Service
	project string
	mutex   sync.RWMutex
}

// Get Project-specific quotas
func (e *Exporter) getProjectQuotas(ch chan<- prometheus.Metric) {
	project, err := e.service.Projects.Get(e.project).Do()
	if err != nil {
		log.Fatalf("Unable to query API: %v", err)
	}
	for _, quota := range project.Quotas {
		ch <- prometheus.MustNewConstMetric(limitDesc, prometheus.GaugeValue, quota.Limit, e.project, "", quota.Metric)
		ch <- prometheus.MustNewConstMetric(usageDesc, prometheus.GaugeValue, quota.Usage, e.project, "", quota.Metric)
	}
}

// Get Region-specific quotas
func (e *Exporter) getRegionQuotas(ch chan<- prometheus.Metric) {
	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		log.Fatalf("Unable to query API: %v", err)
	}
	for _, region := range regionList.Items {
		regionName := region.Name
		for _, quota := range region.Quotas {
			ch <- prometheus.MustNewConstMetric(limitDesc, prometheus.GaugeValue, quota.Limit, e.project, regionName, quota.Metric)
			ch <- prometheus.MustNewConstMetric(usageDesc, prometheus.GaugeValue, quota.Usage, e.project, regionName, quota.Metric)
		}
	}
}

// Describe is implemented with DescribeByCollect. That's possible because the Collect method will always return the same metrics with the same descriptors.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(e, ch)
}

// Collect will run each time the exporter is polled and will in turn call the Google API for the required statistics.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.getProjectQuotas(ch)
	e.getRegionQuotas(ch)
}

// NewExporter returns an initialised Exporter.
func NewExporter(credfile string, project string) (*Exporter, error) {

	// Read credentials from JSON file into a byte array
	var credentials, err = ioutil.ReadFile(credfile)
	if err != nil {
		log.Fatalf("Unable to read credentials file: %v", err)
	}

	// Create context and generate compute.Service
	ctx := context.Background()
	computeService, err := compute.NewService(ctx, option.WithCredentialsJSON(credentials))
	if err != nil {
		log.Fatalf("Unable to create service: %v", err)
	}

	return &Exporter{
		service: computeService,
		project: project,
	}, nil
}

func main() {

	var (
		// Default port added to https://github.com/prometheus/prometheus/wiki/Default-port-allocations
		listenAddress  = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9592").String()
		metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		gceCredentials = kingpin.Flag("gce.credentials-path", "Path to Google Cloud Platform credentials json file.").Default("credentials.json").String()
		gceProjectID   = kingpin.Flag("gce.project-id", "ID of Google Project to be monitored.").Required().String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gce_quota_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gce_quota_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := NewExporter(*gceCredentials, *gceProjectID)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("gce_quota_exporter"))

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>GCE Quota Exporter</title></head>
             <body>
             <h1>GCE Quota Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}
