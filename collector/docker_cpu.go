package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	dockerCpuSubsystem = "docker_containers"
)

type dockerCpuCollector struct {
	client *docker.Client
	cpu    *prometheus.CounterVec
}

func init() {
	Factories["docker_cpu"] = NewDockerCpuCollector
}

func NewDockerCpuCollector() (Collector, error) {
	var labels = []string{"name", "image"}

	client, err := NewDockerClient("")
	if err != nil {
		return nil, err
	}

	return &dockerCpuCollector{
		client: client,
		cpu: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Subsystem: dockerCpuSubsystem,
				Name:      "cpu_usage_seconds_total",
				Help:      "Container combined CPU time in seconds",
			},
			labels,
		),
	}, nil
}

func (c *dockerCpuCollector) Update(ch chan<- prometheus.Metric) error {
	c.cpu.Reset()

	containers, err := c.client.ListContainers(
		docker.ListContainersOptions{All: false},
	)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// remove leading slash
		name := container.Names[0][1:]

		log.Debugf("adding cpu metrics for container %s", name)
		cpu, err := getContainerCpuInfo(container.ID)
		if err != nil {
			log.Debugf("failed to collect cpu metrics for container %s", name)
			continue
		}

		c.cpu.WithLabelValues(name, container.Image).Set(cpu)
	}

	c.cpu.Collect(ch)

	return nil
}

func getContainerCpuInfo(id string) (float64, error) {
	f, err := findDockerCpuStatsFile(id)
	if err != nil {
		return 0, err
	}

	file, err := os.Open(f)
	if err != nil {
		return 0, err
	}

	defer file.Close()

	return parseContainerCpuInfo(file)
}

func parseContainerCpuInfo(r io.Reader) (float64, error) {
	var (
		scanner = bufio.NewScanner(r)
		usage   float64
	)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		value, err := strconv.ParseFloat(line, 64)
		if err != nil {
			return usage, err
		}
		usage = (time.Duration(value) * time.Nanosecond).Seconds()
	}

	return usage, nil
}

func findDockerCpuStatsFile(id string) (string, error) {
	var files = []string{
		sysFilePath(fmt.Sprintf("fs/cgroup/cpuacct/docker/%s/cpuacct.usage", id)),
		sysFilePath(fmt.Sprintf("fs/cgroup/cpuacct/system.slice/docker-%s.scope/cpuacct.usage", id)),
	}

	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			return file, err
		}
	}

	return "", fmt.Errorf("failed to find CPU stats file for container %s", id)
}
