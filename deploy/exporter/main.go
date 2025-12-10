package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Metric definition
	containerMeta = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "container_meta_info",
			Help: "Container metadata info",
		},
		[]string{"id", "name", "image", "com_docker_compose_service", "state", "full_id"},
	)
)

func init() {
	// Register metric with Prometheus
	prometheus.MustRegister(containerMeta)
}

func collectMetrics() {
	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating Docker client: %v", err)
		return
	}
	defer cli.Close()

	// Get list of all containers
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		log.Printf("Error listing containers: %v", err)
		return
	}

	// Reset metric to clear old data
	containerMeta.Reset()

	for _, container := range containers {
		// Extract IDs
		fullID := container.ID
		shortID := fullID
		if len(fullID) > 12 {
			shortID = fullID[:12]
		}

		// Extract Name (remove leading slash)
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}

		// Extract Image
		image := container.Image

		// Extract Service Label
		service := container.Labels["com.docker.compose.service"]
		if service == "" {
			service = name // Fallback
		}

		containerMeta.WithLabelValues(
			shortID,
			name,
			image,
			service,
			container.State,
			fullID,
		).Set(1)
	}
}

func main() {
	// Start metric collection goroutine
	go func() {
		for {
			collectMetrics()
			time.Sleep(15 * time.Second)
		}
	}()

	// Expose metrics via HTTP
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Starting Docker Meta Exporter on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
