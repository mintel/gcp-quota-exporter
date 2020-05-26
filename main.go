package main

import (
	"context"
	"encoding/json"
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

var (
	limitDesc = prometheus.NewDesc("gcp_quota_limit", "quota limits for GCP components", []string{"project", "region", "metric"}, nil)
	usageDesc = prometheus.NewDesc("gcp_quota_usage", "quota usage for GCP components", []string{"project", "region", "metric"}, nil)
	upDesc    = prometheus.NewDesc("up", "Was the last scrape of the Google API successful.", nil, nil)
)

// Exporter collects quota stats from the Google Compute API and exports them using the Prometheus metrics package.
type Exporter struct {
	service *compute.Service
	project string
	mutex   sync.RWMutex
}

// GCP service accounts contain the project id
type ServiceAccount struct {
	ProjectId string `json:"project_id"`
}

// scrape connects to the Google API to retreive quota statistics and record them as metrics.
func (e *Exporter) scrape() (up float64, prj *compute.Project, rgl *compute.RegionList) {

	project, err := e.service.Projects.Get(e.project).Do()
	if err != nil {
		log.Errorf("Failure when querying project quotas: %v", err)
		return 0, nil, nil
	}

	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		log.Errorf("Failure when querying region quotas: %v", err)
		return 0, nil, nil
	}

	return 1, project, regionList
}

// Describe is implemented with DescribeByCollect. That's possible because the
// Collect method will always return the same metrics with the same descriptors.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(e, ch)
}

// Collect will run each time the exporter is polled and will in turn call the
// Google API for the required statistics.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	up, project, regionList := e.scrape()

	for _, quota := range project.Quotas {
		ch <- prometheus.MustNewConstMetric(limitDesc, prometheus.GaugeValue, quota.Limit, e.project, "", quota.Metric)
		ch <- prometheus.MustNewConstMetric(usageDesc, prometheus.GaugeValue, quota.Usage, e.project, "", quota.Metric)
	}

	for _, region := range regionList.Items {
		regionName := region.Name
		for _, quota := range region.Quotas {
			ch <- prometheus.MustNewConstMetric(limitDesc, prometheus.GaugeValue, quota.Limit, e.project, regionName, quota.Metric)
			ch <- prometheus.MustNewConstMetric(usageDesc, prometheus.GaugeValue, quota.Usage, e.project, regionName, quota.Metric)
		}
	}

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, up)
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
		gcpProjectID   = kingpin.Arg("gcp_project_id", "ID of Google Project to be monitored.").String()
		listenAddress  = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9592").String()
		metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		gcpCredentials = kingpin.Flag("gcp.credentials-path", "Path to Google Cloud Platform credentials json file.").Default("credentials.json").String()
		basePath       = kingpin.Flag("test.base-path", "Change the default googleapis URL (for testing purposes only).").Default("").String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gcp_quota_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gcp_quota_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	if *gcpProjectID == "" {
		var svcAcct ServiceAccount

		serviceContent, err := ioutil.ReadFile(*gcpCredentials)
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal([]byte(serviceContent), &svcAcct)
		if err != nil {
			log.Fatalf("Unable to parse file: %v", err)
		}

		if svcAcct.ProjectId == "" {
			log.Fatalf("Unable to parse project_id from %s", *gcpCredentials)
		}

		*gcpProjectID = svcAcct.ProjectId
	}

	exporter, err := NewExporter(*gcpCredentials, *gcpProjectID)
	if err != nil {
		log.Fatal(err)
	}

	if *basePath != "" {
		exporter.service.BasePath = *basePath
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("gcp_quota_exporter"))

	log.Infoln("Listening on", *listenAddress)
	log.Infoln("GCP Project: ", *gcpProjectID)
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
