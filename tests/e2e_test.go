package resourcestatemetrics_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rexagod/resource-state-metrics/tests/framework"
)

func TestMainServer(t *testing.T) {
	t.Parallel()

	mainPort, found := os.LookupEnv(MainPort)
	if !found {
		t.Fatal(MainPort + " is not set")
	}
	mainMetricsURL := &url.URL{
		Host:   "localhost:" + mainPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	wantRawBytes, err := os.ReadFile("metrics")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	wantRaw := string(wantRawBytes)
	if err = testutil.ScrapeAndCompare(mainMetricsURL.String(), strings.NewReader(wantRaw)); err != nil {
		t.Fatal(err)
	}
}

func TestExternalMainServer(t *testing.T) {
	t.Parallel()

	mainPort, found := os.LookupEnv(MainPort)
	if !found {
		t.Fatal(MainPort + "is not set")
	}
	externalMainMetricsURL := &url.URL{
		Host:   "localhost:" + mainPort,
		Path:   "/external",
		Scheme: "http",
	}
	wantRawBytes, err := os.ReadFile("external")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	wantRaw := string(wantRawBytes)
	if err = testutil.ScrapeAndCompare(externalMainMetricsURL.String(), strings.NewReader(wantRaw)); err != nil {
		t.Fatal(err)
	}
}

func TestSelfServer(t *testing.T) {
	t.Parallel()

	runner := framework.NewRunner()
	const httpRequestDurationSeconds = "http_request_duration_seconds"

	// Fetch the recorded in-flight time for main /metrics endpoint.
	selfPort, found := os.LookupEnv(SelfPort)
	if !found {
		t.Fatal(SelfPort + "is not set")
	}
	selfMetricsURL := &url.URL{
		Host:   "localhost:" + selfPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	telemetryMetrics, err := runner.GetMetrics(selfMetricsURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}
	inFlightDurationTotal := 0.0
	inFlightDurationFamily, ok := telemetryMetrics[httpRequestDurationSeconds]
	if ok {
		inFlightDurationTotal = inFlightDurationFamily.GetMetric()[0].GetHistogram().GetSampleSum()
	}

	// Ping main /metrics endpoint.
	mainPort, found := os.LookupEnv("RSM_MAIN_PORT")
	if !found {
		t.Fatal("RSM_MAIN_PORT is not set")
	}
	mainURL := &url.URL{
		Host:   "localhost:" + mainPort,
		Path:   "/metrics",
		Scheme: "http",
	}
	_, err = runner.GetRaw(mainURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	// Check if the recorded in-flight time for main /metrics requests increased.
	telemetryMetrics, err = runner.GetMetrics(selfMetricsURL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}
	newInFlightDurationTotal := telemetryMetrics[httpRequestDurationSeconds].GetMetric()[0].GetHistogram().GetSampleSum()
	if newInFlightDurationTotal == inFlightDurationTotal {
		t.Fatalf("got in-flight duration total %f, want %f", newInFlightDurationTotal, inFlightDurationTotal)
	}
}
