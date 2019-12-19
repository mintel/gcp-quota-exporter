package main

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	limitDesc = prometheus.NewDesc("gcp_quota_limit", "quota limits for GCP components", []string{"project", "region", "metric"}, nil)
	usageDesc = prometheus.NewDesc("gcp_quota_usage", "quota usage for GCP components", []string{"project", "region", "metric"}, nil)
)

// Exporter collects quota stats from the Google Compute API and exports them using the Prometheus metrics package.
type Exporter struct {
	service      *compute.Service
	project      string
	mutex        sync.RWMutex
	backoffLimit int
}

// Get Project-specific quotas
func (e *Exporter) getProjectQuotas(ch chan<- prometheus.Metric) {

	var project *compute.Project
	var err error
	var millis time.Duration

	for n := 0; n <= e.backoffLimit-1; n++ {
		project, err = e.service.Projects.Get(e.project).Do()
		if err == nil {
			break
		} else {
			log.Errorf("Unable to query API: %v. Retrying (%v / %v).", err, n+1, e.backoffLimit)
			millis = time.Duration(rand.Int31n(1000)) * time.Millisecond
			time.Sleep(time.Duration(2^n) + millis)
		}
	}
	if err != nil {
		log.Fatal()
	}
	for _, quota := range project.Quotas {
		ch <- prometheus.MustNewConstMetric(limitDesc, prometheus.GaugeValue, quota.Limit, e.project, "", quota.Metric)
		ch <- prometheus.MustNewConstMetric(usageDesc, prometheus.GaugeValue, quota.Usage, e.project, "", quota.Metric)
	}
}

// Get Region-specific quotas
func (e *Exporter) getRegionQuotas(ch chan<- prometheus.Metric) {

	var regionList *compute.RegionList
	var err error
	var millis time.Duration

	for n := 0; n <= e.backoffLimit-1; n++ {
		regionList, err = e.service.Regions.List(e.project).Do()
		if err == nil {
			break
		} else {
			log.Errorf("Unable to query API: %v. Retrying (%v / %v).", err, n+1, e.backoffLimit)
			millis = time.Duration(rand.Int31n(1000)) * time.Millisecond
			time.Sleep(time.Duration(2^n) + millis)
		}
	}
	if err != nil {
		log.Fatal()
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
func NewExporter(credfile string, project string, backoffLimit int) (*Exporter, error) {

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
		service:      computeService,
		project:      project,
		backoffLimit: backoffLimit,
	}, nil
}

func main() {

	var (
		// Default port added to https://github.com/prometheus/prometheus/wiki/Default-port-allocations
		gcpProjectID = kingpin.Arg("gcp_project_id", "ID of Google Project to be monitored.").Required().String()
		//gcpServices    = kingpin.Arg("gcp_services", "Comma-separated list of service names to monitor as per `gcloud services list | awk '{print $1}' | sed 's/\\.googleapis\\.com//g'`)").Required().String()
		listenAddress  = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9592").String()
		metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		gcpCredentials = kingpin.Flag("gcp.credentials-path", "Path to Google Cloud Platform credentials json file.").Default("credentials.json").String()
		backoffLimit   = kingpin.Flag("backoff-limit", "How many times the app will retry API connections when an error response is recieved from Google.").Default("13").Int()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gcp_quota_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gcp-quota-exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := NewExporter(*gcpCredentials, *gcpProjectID, *backoffLimit)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("gcp_quota_exporter"))

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>GCP Quota Exporter</title></head>
             <body>
             <h1>GCP Quota Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}
