# Google Cloud Platform Quota Exporter

Exports limits and usage for metrics available through the GCP APIs (currently only supports Compute Engine).

## Usage

1. Set up a service account in the project you wish to monitor. The account should be given the following permissions:
  * compute.projects.get
  * compute.regions.list
1. Authentication is performed using the standard [Application Default Credentials](https://developers.google.com/accounts/docs/application-default-credentials)
  * To use a credentials.json key file export the environment variable `GOOGLE_APPLICATION_CREDENTIALS=path-to-credentials.json`
1. The exporter need to know which project to monitor quotas for
  * Specify project using `--gcp.project_id`  
  * Export environment variable `GOOGLE_PROJECT_ID`
  * Fetch from compute metadata `http://metadata.google.internal/computeMetadata/v1/project/project-id`

## Docker-compose

1. Copy the example file and add your project id to it
1. Change the volume to point to your credentials file if different
1. Run `docker-compose up` and you'll have a prometheus instance running at http://localhost:9090 and a gcp-quota-exporter instance running at http://localhost:9592.

## Docker

### Local Build

```
docker build -t gcp-quota-exporter .
docker run -it --rm -v $(pwd)/credentials.json:/app/credentials.json -e GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json -e GOOGLE_PROJECT_ID=project_id gcp-quota-exporter
```

### Official Build

```
docker run -it --rm -v $(pwd)/credentials.json:/app/credentials.json -e GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json -e GOOGLE_PROJECT_ID=project_id mintel/gcp-quota-exporter
```
