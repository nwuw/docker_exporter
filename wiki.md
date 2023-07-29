# 导入依赖库

首先，我们导入了需要使用的依赖库，包括 Prometheus 客户端库、Docker 客户端库以及其他标准库。这些库将帮助我们实现 Exporter 并与 Docker API 进行交互。

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)
```

# 定义指标描述符

 我们定义了两个 Prometheus 指标描述符 `cpuUsageDesc` 和 `memoryUsageDesc`，用于描述要暴露的指标。每个指标都有一个唯一的名称、帮助文本和标签集。在本例中，我们为容器的 CPU 使用率和内存使用率定义了指标。

```go
const (
	namespace = "docker_container"
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
```

# 定义自定义 Exporter 结构体

 我们创建了一个 `dockerCollector` 结构体，用于实现 Prometheus 的 `Collector` 接口。在结构体中，我们存储了与 Docker API 的连接客户端。

```go
type dockerCollector struct {
	dockerClient *client.Client
}
```

# 初始化 Docker 客户端

我们使用 `newDockerCollector` 函数创建了一个新的 `dockerCollector` 实例，并初始化了 Docker 客户端连接。

```go
func newDockerCollector() (*dockerCollector, error) {
	cli, err := client.NewClientWithOpts(client.WithVersion("1.41")) // Use the appropriate Docker API version
	if err != nil {
		return nil, err
	}

	return &dockerCollector{
		dockerClient: cli,
	}, nil
}
```

# 实现 Collector 接口的 Describe 方法

我们需要实现 Prometheus 的 `Collector` 接口中的 `Describe` 方法，它用于向 Prometheus 描述我们要暴露的指标。在本例中，我们只需向通道中发送定义的指标描述符即可。

```go
func (dc *dockerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cpuUsageDesc
	ch <- memoryUsageDesc
}
```

# 实现 Collector 接口的 Collect 方法

 `Collect` 方法用于收集指标数据，并将其发送到 Prometheus。在这个方法中，我们首先获取所有容器的列表，然后遍历每个容器，获取其 CPU 使用率和内存使用率，并通过通道将这些数据发送给 Prometheus。

```go
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
```

# 获取容器的指标数据

`getContainerMetrics` 函数使用 Docker 客户端获取容器的统计数据，并根据这些数据计算容器的 CPU 使用率和内存使用率。

```go
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
```

# 启动 HTTP 服务器并注册 Collector

在 `main` 函数中，我们创建了一个新的 `dockerCollector` 实例，并将其注册到 Prometheus 中。然后，我们使用 `promhttp.Handler()` 将指标暴露为 HTTP 服务，并在端口 8080 上监听请求。

```go
func main() {
	dc, err := newDockerCollector()
	if err != nil {
		log.Fatal("Error creating Docker collector:", err)
	}

	prometheus.MustRegister(dc)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

