.PHONY : build
build : gcp-quota-exporter

gcp-quota-exporter : main.go
	@echo "building go binary"
	@GOOS=linux go build -o ./gcp-quota-exporter .

.PHONY : unit-test
unit-test :
	@echo "unit-test placeholder"

.PHONY : func-test
func-test :
	@echo "func-test placeholder"