.PHONY : build
build : gcp-quota-exporter

gcp-quota-exporter : main.go
	@echo "building go binary"
	@CGO_ENABLED=0 GOOS=linux go build .
