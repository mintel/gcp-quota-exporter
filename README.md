# Google Cloud Platform Quota Exporter

Exports limits and usage for metrics available through the GCP APIs (currently only supports Compute Engine).

## Usage

1. Set up a service account with Compute Viewer access to the project you wish to monitor
1. Create a key for the service account and save as a JSON somewhere (by default the exporter will look for `./credentials.json`)
1. Run it and provide a project name:
```bash
./gcp-quota-exporter myproject
```

## Docker-compose

1. Copy the example file and add your project id to it
1. Change the volume to point to your credentials file if different
1. Run `docker-compose up` and you'll have a prometheus instance running at http://localhost:9090 and a gcp-quota-exporter instance running at http://localhost:9592.

## Docker

### Local Build

```
docker build -t gcp-quota-exporter .
docker run -it --rm -v $(pwd)/credentials.json:/app/credentials.json gcp-quota-exporter myproject
```

### Official Build

```
docker run -it --rm -v $(pwd)/credentials.json:/app/credentials.json mintel/gcp-quota-exporter myproject
```