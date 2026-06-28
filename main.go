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
// /v1/stats/users types
// ---------------------------------------------------------------------------

type User struct {
	Username           string `json:"username"`
	Enabled            bool   `json:"enabled"`
	CurrentConnections int64  `json:"current_connections"`
	TotalOctets        int64  `json:"total_octets"`
}

type UsersResponse struct {
	Ok       bool   `json:"ok"`
	Data     []User `json:"data"`
	Revision string `json:"revision"`
}

// ---------------------------------------------------------------------------
// /v1/stats/zero/all types
// ---------------------------------------------------------------------------

type ClassCount struct {
	Class string `json:"class"`
	Total int64  `json:"total"`
}

type ZeroCoreData struct {
	ConnectionsTotal          int64        `json:"connections_total"`
	ConnectionsBadTotal       int64        `json:"connections_bad_total"`
	ConnectionsBadByClass     []ClassCount `json:"connections_bad_by_class"`
	HandshakeFailuresByClass  []ClassCount `json:"handshake_failures_by_class"`
	HandshakeTimeoutsTotal    int64        `json:"handshake_timeouts_total"`
	AcceptPermitTimeoutTotal  int64        `json:"accept_permit_timeout_total"`
	ConfiguredUsers           int          `json:"configured_users"`
}

type ZeroAllData struct {
	Core ZeroCoreData `json:"core"`
}

type ZeroAllResponse struct {
	Ok       bool        `json:"ok"`
	Data     ZeroAllData `json:"data"`
	Revision string      `json:"revision"`
}

// ---------------------------------------------------------------------------
// Collector
// ---------------------------------------------------------------------------

type TelemtCollector struct {
	baseURL string
	token   string
	client  *http.Client

	// From /v1/stats/users
	userTraffic     *prometheus.Desc
	userConnections *prometheus.Desc

	// From /v1/stats/zero/all
	connectionsTotal          *prometheus.Desc
	connectionsBadTotal       *prometheus.Desc
	connectionsBadByClass     *prometheus.Desc
	handshakeFailuresByClass  *prometheus.Desc

	// Scrape status
	scrapeUp *prometheus.Desc
}

func NewTelemtCollector(baseURL, token string) *TelemtCollector {
	return &TelemtCollector{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},

		userTraffic: prometheus.NewDesc(
			"telemt_user_traffic_bytes_total",
			"Total bytes transferred by user (upload + download)",
			[]string{"username"}, nil,
		),
		userConnections: prometheus.NewDesc(
			"telemt_user_active_connections",
			"Current active connections for user",
			[]string{"username"}, nil,
		),

		connectionsTotal: prometheus.NewDesc(
			"telemt_connections_total",
			"Total connection attempts (good + bad) since process start",
			nil, nil,
		),
		connectionsBadTotal: prometheus.NewDesc(
			"telemt_connections_bad_total",
			"Total failed connection attempts since process start",
			nil, nil,
		),
		connectionsBadByClass: prometheus.NewDesc(
			"telemt_connections_bad_by_class",
			"Failed connection attempts grouped by failure class",
			[]string{"class"}, nil,
		),
		handshakeFailuresByClass: prometheus.NewDesc(
			"telemt_handshake_failures_by_class",
			"Handshake failures grouped by failure class",
			[]string{"class"}, nil,
		),

		scrapeUp: prometheus.NewDesc(
			"telemt_scrape_up",
			"1 if last scrape of telemt API succeeded",
			nil, nil,
		),
	}
}

func (c *TelemtCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.userTraffic
	ch <- c.userConnections
	ch <- c.connectionsTotal
	ch <- c.connectionsBadTotal
	ch <- c.connectionsBadByClass
	ch <- c.handshakeFailuresByClass
	ch <- c.scrapeUp
}

func (c *TelemtCollector) Collect(ch chan<- prometheus.Metric) {
	allOk := true

	users, err := c.fetchUsers()
	if err != nil {
		log.Printf("ERROR fetching /v1/stats/users: %v", err)
		allOk = false
	} else {
		for _, u := range users {
			ch <- prometheus.MustNewConstMetric(
				c.userTraffic, prometheus.CounterValue, float64(u.TotalOctets), u.Username,
			)
			ch <- prometheus.MustNewConstMetric(
				c.userConnections, prometheus.GaugeValue, float64(u.CurrentConnections), u.Username,
			)
		}
	}

	zero, err := c.fetchZeroAll()
	if err != nil {
		log.Printf("ERROR fetching /v1/stats/zero/all: %v", err)
		allOk = false
	} else {
		ch <- prometheus.MustNewConstMetric(
			c.connectionsTotal, prometheus.CounterValue, float64(zero.Core.ConnectionsTotal),
		)
		ch <- prometheus.MustNewConstMetric(
			c.connectionsBadTotal, prometheus.CounterValue, float64(zero.Core.ConnectionsBadTotal),
		)
		for _, cc := range zero.Core.ConnectionsBadByClass {
			ch <- prometheus.MustNewConstMetric(
				c.connectionsBadByClass, prometheus.CounterValue, float64(cc.Total), cc.Class,
			)
		}
		for _, fc := range zero.Core.HandshakeFailuresByClass {
			ch <- prometheus.MustNewConstMetric(
				c.handshakeFailuresByClass, prometheus.CounterValue, float64(fc.Total), fc.Class,
			)
		}
	}

	scrapeVal := 0.0
	if allOk {
		scrapeVal = 1.0
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeUp, prometheus.GaugeValue, scrapeVal)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (c *TelemtCollector) fetchUsers() ([]User, error) {
	url := fmt.Sprintf("%s/v1/stats/users", c.baseURL)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result UsersResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	if !result.Ok {
		return nil, fmt.Errorf("telemt API returned ok=false")
	}
	return result.Data, nil
}

func (c *TelemtCollector) fetchZeroAll() (*ZeroAllData, error) {
	url := fmt.Sprintf("%s/v1/stats/zero/all", c.baseURL)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}

	var result ZeroAllResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	if !result.Ok {
		return nil, fmt.Errorf("telemt API returned ok=false")
	}
	return &result.Data, nil
}

func (c *TelemtCollector) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", c.token)
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
	return body, nil
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
