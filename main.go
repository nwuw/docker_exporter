package main

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

const (
	namespace = "docker_exporter"
)

var (
	cpuUsageDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cpu_usage_percent"),
		"Container CPU Usage Percentage",
		[]string{"container_id"}, nil,
	)

	memoryUsageDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "memory_usage_bytes"),
		"Container Memory Usage in bytes",
		[]string{"container_id"}, nil,
	)
)

type dockerCollector struct {
	dockerClient *client.Client
}

func newDockerCollector() (*dockerCollector, error) {
	cli, err := client.NewClientWithOpts(client.WithVersion("1.41")) // Use the appropriate Docker API version
	if err != nil {
		return nil, err
	}

	return &dockerCollector{
		dockerClient: cli,
	}, nil
}

func (dc *dockerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cpuUsageDesc
	ch <- memoryUsageDesc
}

func (dc *dockerCollector) Collect(ch chan<- prometheus.Metric) {
	containers, err := dc.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Println("Failed to list containers:", err)
		return
	}

	for _, container := range containers {
		cpuUsagePercent, memoryUsageBytes, err := dc.getContainerMetrics(container.ID)
		if err != nil {
			log.Println("Failed to get metrics for container", container.ID, ":", err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(cpuUsageDesc, prometheus.GaugeValue, cpuUsagePercent, container.ID)
		ch <- prometheus.MustNewConstMetric(memoryUsageDesc, prometheus.GaugeValue, float64(memoryUsageBytes), container.ID)
	}
}

func (dc *dockerCollector) getContainerMetrics(containerID string) (float64, uint64, error) {
	stats, err := dc.dockerClient.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		return 0, 0, err
	}
	defer stats.Body.Close()

	var statData types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&statData); err != nil {
		return 0, 0, err
	}

	// Calculate CPU usage percentage
	cpuDelta := float64(statData.CPUStats.CPUUsage.TotalUsage - statData.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(statData.CPUStats.SystemUsage - statData.PreCPUStats.SystemUsage)
	cpuUsagePercent := (cpuDelta / systemDelta) * float64(len(statData.CPUStats.CPUUsage.PercpuUsage)) * 100.0

	// Memory usage in bytes
	memoryUsageBytes := statData.MemoryStats.Usage - statData.MemoryStats.Stats["cache"]

	return cpuUsagePercent, memoryUsageBytes, nil
}

func main() {
	dc, err := newDockerCollector()
	if err != nil {
		log.Fatal("Error creating Docker collector:", err)
	}

	prometheus.MustRegister(dc)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":924", nil))
}
