package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	dockerSubsystem = "docker"
)

type memoryStats map[string]float64
type networkStats map[string]float64

type dockerCollector struct {
	containers, memory *prometheus.GaugeVec
	cpu                *prometheus.CounterVec
	client             *docker.Client
}

func init() {
	Factories["docker"] = NewDockerCollector
}

func NewDockerCollector() (Collector, error) {
	var containerLabelNames = []string{"name", "id", "image"}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		return &dockerCollector{}, err
	}

	return &dockerCollector{
		client: client,
		containers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: dockerSubsystem,
				Name:      "containers",
				Help:      "nothing yet",
			},
			containerLabelNames,
		),
		memory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: dockerSubsystem,
				Name:      "memory_usage_bytes",
				Help:      "Container memory usage in bytes",
			},
			containerLabelNames,
		),
		cpu: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Subsystem: dockerSubsystem,
				Name:      "cpu_usage_seconds_total",
				Help:      "Container combined CPU time in seconds",
			},
			containerLabelNames,
		),
	}, nil
}

func (c *dockerCollector) Update(ch chan<- prometheus.Metric) error {
	c.reset()

	containers, err := c.client.ListContainers(
		docker.ListContainersOptions{All: false},
	)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// remove leading slash
		name := container.Names[0][1:]

		log.Debugf("adding container %s with id %s", name, container.ID)
		c.containers.WithLabelValues(name, container.ID, container.Image).Set(1)

		log.Debugf("gathering memory usage for container %s", name)
		mem, err := getMemoryInfo(container.ID)
		if err == nil {
			log.Debugf("adding memory usage for %s", name)
			c.memory.WithLabelValues(name, container.ID, container.Image).Set(memoryUsageBytes(mem))
		} else {
			log.Debugf("failed to gather memory usage: %s", err)
		}

		log.Debugf("gathering cpu usage for container %s", name)
		cpu, err := getCpuInfo(container.ID)
		if err == nil {
			log.Debugf("adding cpu usage for %s", name)
			c.cpu.WithLabelValues(name, container.ID, container.Image).Set(cpu)
		} else {
			log.Debugf("failed to gather cpu usage: %s", err)
		}
	}

	c.containers.Collect(ch)
	c.memory.Collect(ch)
	c.cpu.Collect(ch)

	return err
}

func getMemoryInfo(id string) (memoryStats, error) {
	file, err := os.Open(sysFilePath(fmt.Sprintf("fs/cgroup/memory/docker/%s/memory.stat", id)))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return parseMemoryInfo(file)
}

func parseMemoryInfo(r io.Reader) (memoryStats, error) {
	var (
		memory  = memoryStats{}
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

func memoryUsageBytes(m memoryStats) float64 {
	return m["rss"] + m["cache"]
}

func getCpuInfo(id string) (float64, error) {
	file, err := os.Open(sysFilePath(fmt.Sprintf("fs/cgroup/cpuacct/docker/%s/cpuacct.usage", id)))
	if err != nil {
		return 0, err
	}

	defer file.Close()

	return parseCpuInfo(file)
}

func parseCpuInfo(r io.Reader) (float64, error) {
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

func (c *dockerCollector) reset() {
	c.containers.Reset()
	c.memory.Reset()
	c.cpu.Reset()
}
