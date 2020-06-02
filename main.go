package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"

	"cloud.google.com/go/compute/metadata"
	"github.com/PuerkitoBio/rehttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/tidwall/gjson"
)

var (
	limitDesc = prometheus.NewDesc("gcp_quota_limit", "quota limits for GCP components", []string{"project", "region", "metric"}, nil)
	usageDesc = prometheus.NewDesc("gcp_quota_usage", "quota usage for GCP components", []string{"project", "region", "metric"}, nil)
	upDesc    = prometheus.NewDesc("up", "Was the last scrape of the Google API successful.", nil, nil)

	gcpProjectID = kingpin.Flag(
		"gcp.project_id", "ID of the Google Project to be monitored. ($GOOGLE_PROJECT_ID)",
	).Envar("GOOGLE_PROJECT_ID").String()

	gcpMaxRetries = kingpin.Flag(
		"gcp.max-retries", "Max number of retries that should be attempted on 503 errors from gcp. ($GCP_EXPORTER_MAX_RETRIES)",
	).Envar("GCP_EXPORTER_MAX_RETRIES").Default("0").Int()

	gcpHttpTimeout = kingpin.Flag(
		"gcp.http-timeout", "How long should gcp_exporter wait for a result from the Google API ($GCP_EXPORTER_HTTP_TIMEOUT)",
	).Envar("GCP_EXPORTER_HTTP_TIMEOUT").Default("10s").Duration()

	gcpMaxBackoffDuration = kingpin.Flag(
		"gcp.max-backoff", "Max time between each request in an exp backoff scenario ($GCP_EXPORTER_MAX_BACKOFF_DURATION)",
	).Envar("GCP_EXPORTER_MAX_BACKOFF_DURATION").Default("5s").Duration()

	gcpBackoffJitterBase = kingpin.Flag(
		"gcp.backoff-jitter", "The amount of jitter to introduce in a exp backoff scenario ($GCP_EXPORTER_BACKODFF_JITTER_BASE)",
	).Envar("GCP_EXPORTER_BACKOFF_JITTER_BASE").Default("1s").Duration()

	gcpRetryStatuses = kingpin.Flag(
		"gcp.retry-statuses", "The HTTP statuses that should trigger a retry ($GCP_EXPORTER_RETRY_STATUSES)",
	).Envar("GCP_EXPORTER_RETRY_STATUSES").Default("503").Ints()
)

// Exporter collects quota stats from the Google Compute API and exports them using the Prometheus metrics package.
type Exporter struct {
	service *compute.Service
	project string
	mutex   sync.RWMutex
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
func NewExporter(project string) (*Exporter, error) {
	// Create context and generate compute.Service
	ctx := context.Background()

	googleClient, err := google.DefaultClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("Error creating Google client: %v", err)
	}

	googleClient.Timeout = *gcpHttpTimeout
	googleClient.Transport = rehttp.NewTransport(
		googleClient.Transport, // need to wrap DefaultClient transport
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(*gcpMaxRetries),
			rehttp.RetryStatuses(*gcpRetryStatuses...)), // Cloud support suggests retrying on 503 errors
		rehttp.ExpJitterDelay(*gcpBackoffJitterBase, *gcpMaxBackoffDuration), // Set timeout to <10s as that is prom default timeout
	)

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(googleClient))
	if err != nil {
		log.Fatalf("Unable to create service: %v", err)
	}

	return &Exporter{
		service: computeService,
		project: project,
	}, nil
}

func GetProjectIdFromMetadata() (string, error) {
	client := metadata.NewClient(&http.Client{})

	project_id, err := client.ProjectID()
	if err != nil {
		return "", err
	}

	return project_id, nil
}

func main() {

	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9592").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		basePath      = kingpin.Flag("test.base-path", "Change the default googleapis URL (for testing purposes only).").Default("").String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gcp_quota_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gcp_quota_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	// Detect Project ID
	if *gcpProjectID == "" {
		credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

		if credentialsFile != "" {
			c, err := ioutil.ReadFile(credentialsFile)
			if err != nil {
				log.Fatalf("Unable to read %s: %v", credentialsFile, err)
			}

			projectId := gjson.GetBytes(c, "project_id")

			if projectId.String() == "" {
				log.Fatalf("Could not retrieve Project ID from %s", credentialsFile)
			}

			*gcpProjectID = projectId.String()
		} else {
			project_id, err := GetProjectIdFromMetadata()
			if err != nil {
				log.Fatal(err)
			}

			*gcpProjectID = project_id
		}
	}

	if *gcpProjectID == "" {
		log.Fatal("GCP Project ID cannot be empty")
	}

	exporter, err := NewExporter(*gcpProjectID)
	if err != nil {
		log.Fatal(err)
	}

	if *basePath != "" {
		exporter.service.BasePath = *basePath
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("gcp_quota_exporter"))

	log.Infoln("Google Project:", *gcpProjectID)
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
