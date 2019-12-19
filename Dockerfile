FROM golang:1.12.5-alpine3.9 as alpine

RUN apk add --no-cache git ca-certificates make

ENV GO111MODULE=on
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN make build

FROM scratch
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine /app/gcp-quota-exporter /app/
WORKDIR /app
EXPOSE 9592
ENTRYPOINT ["./gcp-quota-exporter"]
