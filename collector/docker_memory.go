package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	dockerMemorySubsystem = "docker_containers"
)

type dockerMemoryCollector struct {
	client *docker.Client
	memory *prometheus.GaugeVec
}

type memoryStatistics map[string]float64

func init() {
	Factories["docker_memory"] = NewDockerMemoryCollector
}

func NewDockerMemoryCollector() (Collector, error) {
	var labels = []string{"name", "image"}

	client, err := NewDockerClient("")
	if err != nil {
		return nil, err
	}

	return &dockerMemoryCollector{
		client: client,
		memory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: dockerMemorySubsystem,
				Name:      "memory_usage_bytes",
				Help:      "Container memory usage in bytes",
			},
			labels,
		),
	}, nil
}

func (c *dockerMemoryCollector) Update(ch chan<- prometheus.Metric) error {
	c.memory.Reset()

	containers, err := c.client.ListContainers(
		docker.ListContainersOptions{All: false},
	)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// remove trailing slash
		name := container.Names[0][1:]

		log.Debugf("adding memory metrics for container %s", name)
		mem, err := getContainerMemoryInfo(container.ID)
		if err != nil {
			log.Debugf("failed to collect memory metrics for container %s: %s", name, err)
			continue
		}

		c.memory.WithLabelValues(name, container.Image).Set(containerMemoryUsageBytes(mem))
	}

	c.memory.Collect(ch)

	return nil
}

func getContainerMemoryInfo(id string) (memoryStatistics, error) {
	f, err := findDockerMemoryStatsFile(id)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(f)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return parseContainerMemoryInfo(file)
}

func parseContainerMemoryInfo(r io.Reader) (memoryStatistics, error) {
	var (
		memory  = memoryStatistics{}
		scanner = bufio.NewScanner(r)
	)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Fields(string(line))

		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, err
		}

		memory[parts[0]] = value
	}

	return memory, nil
}

func containerMemoryUsageBytes(m memoryStatistics) float64 {
	return m["rss"] + m["cache"]
}

func findDockerMemoryStatsFile(id string) (string, error) {
	var files = []string{
		sysFilePath(fmt.Sprintf("fs/cgroup/memory/docker/%s/memory.stat", id)),
		sysFilePath(fmt.Sprintf("fs/cgroup/memory/system.slice/docker-%s.scope/memory.stat", id)),
	}

	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			return file, nil
		}
	}

	return "", fmt.Errorf("failed to find memory stats file for containers %s", id)
}
