FROM golang

RUN go get -u -v \
    github.com/prometheus/client_golang/prometheus \
	github.com/prometheus/client_golang/prometheus/promhttp \
	github.com/prometheus/common/log \
	github.com/prometheus/common/version \
	google.golang.org/api/compute/v1 \
	google.golang.org/api/option \
	gopkg.in/alecthomas/kingpin.v2

EXPOSE 9592

WORKDIR /go/src/app
COPY . .
RUN go install -v .

ENTRYPOINT ["app"]