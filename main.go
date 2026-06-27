package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------------------------------------------------------------------------
// telemt API types
// Подгони под реальные поля своего /api/users или /api/inbounds
// ---------------------------------------------------------------------------

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	UpBytes     int64  `json:"up"`     // байт отправлено
	DownBytes   int64  `json:"down"`   // байт получено
	Connections int    `json:"connections"` // активные соединения
	Enabled     bool   `json:"enable"`
}

type TelemtResponse struct {
	Success bool   `json:"success"`
	Users   []User `json:"obj"` // telemt-ui обычно кладёт список в поле "obj"
}

// ---------------------------------------------------------------------------
// Коллектор
// ---------------------------------------------------------------------------

type TelemtCollector struct {
	baseURL  string
	token    string
	client   *http.Client

	upBytes     *prometheus.Desc
	downBytes   *prometheus.Desc
	connections *prometheus.Desc
	scrapeUp    *prometheus.Desc
}

func NewTelemtCollector(baseURL, token string) *TelemtCollector {
	labels := []string{"user_id", "username"}
	return &TelemtCollector{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},

		upBytes: prometheus.NewDesc(
			"telemt_user_traffic_up_bytes_total",
			"Total bytes sent (upload) by user",
			labels, nil,
		),
		downBytes: prometheus.NewDesc(
			"telemt_user_traffic_down_bytes_total",
			"Total bytes received (download) by user",
			labels, nil,
		),
		connections: prometheus.NewDesc(
			"telemt_user_active_connections",
			"Current active connections for user",
			labels, nil,
		),
		scrapeUp: prometheus.NewDesc(
			"telemt_scrape_up",
			"1 if last scrape of telemt API succeeded",
			nil, nil,
		),
	}
}

func (c *TelemtCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upBytes
	ch <- c.downBytes
	ch <- c.connections
	ch <- c.scrapeUp
}

func (c *TelemtCollector) Collect(ch chan<- prometheus.Metric) {
	users, err := c.fetchUsers()
	if err != nil {
		log.Printf("ERROR fetching telemt users: %v", err)
		ch <- prometheus.MustNewConstMetric(c.scrapeUp, prometheus.GaugeValue, 0)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeUp, prometheus.GaugeValue, 1)

	for _, u := range users {
		id := u.ID
		name := u.Username

		ch <- prometheus.MustNewConstMetric(
			c.upBytes, prometheus.CounterValue, float64(u.UpBytes), id, name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.downBytes, prometheus.CounterValue, float64(u.DownBytes), id, name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.connections, prometheus.GaugeValue, float64(u.Connections), id, name,
		)
	}
}

func (c *TelemtCollector) fetchUsers() ([]User, error) {
	url := fmt.Sprintf("%s/api/users", c.baseURL) // ← подгони endpoint

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TelemtResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("telemt API returned success=false")
	}

	return result.Users, nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	listenAddr := flag.String("listen", ":9101", "Address to expose metrics on")
	telemtURL  := flag.String("url", "http://localhost:54321", "telemt base URL")
	token      := flag.String("token", "", "Bearer token for telemt API (if required)")
	flag.Parse()

	collector := NewTelemtCollector(*telemtURL, *token)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><a href="/metrics">Metrics</a></body></html>`)
	})

	log.Printf("telemt-exporter listening on %s", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
