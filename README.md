## Usage

1. Set up a service account with Compute Viewer access to the project you wish to monitor
1. Create a key for the service account and save as a JSON somewhere (by default the exporter will look for `./credentials.json`)
1. Run it and provide a project name:
```bash
./gce_quota_exporter --gce.project-id myproject
```

## Docker

Add your project name to `docker-compose.yml` and run `docker-compose up` and you'll have a prometheus instance running at http://localhost:9090 and a gce_quota_exporter instance running at http://localhost:9592. Or for just an exporter instance (again at http://localhost:9592):

```
docker build -t gce_quota_exporter .
docker run -it --rm gce_quota_exporter --gce.project-id myproject
```