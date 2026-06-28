FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o telemt-exporter .

FROM scratch
COPY --from=builder /app/telemt-exporter /telemt-exporter
EXPOSE 9101
ENTRYPOINT ["/telemt-exporter"]
