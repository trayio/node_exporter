package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	dockerNetworkSubsystem = "docker_containers"
)

var (
	fieldSeparator = regexp.MustCompile("[ :] *")
)

type dockerNetworkCollector struct {
	client            *docker.Client
	receive, transmit *prometheus.CounterVec
}

type direction struct {
	receive, transmit string
}

type network map[string]map[string]*direction

func init() {
	Factories["docker_network"] = NewDockerNetworkCollector
}

func NewDockerNetworkCollector() (Collector, error) {
	var labels = []string{"name", "image", "interface"}

	client, err := NewDockerClient("")
	if err != nil {
		return nil, err
	}

	return &dockerNetworkCollector{
		client: client,
		receive: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Subsystem: dockerNetworkSubsystem,
				Name:      "network_receive_bytes",
				Help:      "Container network receive in bytes",
			},
			labels,
		),
		transmit: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Subsystem: dockerNetworkSubsystem,
				Name:      "network_transmit_bytes",
				Help:      "Container network transmit in bytes",
			},
			labels,
		),
	}, nil
}

func (c dockerNetworkCollector) Update(ch chan<- prometheus.Metric) error {
	c.receive.Reset()
	c.transmit.Reset()

	containers, err := c.client.ListContainers(
		docker.ListContainersOptions{All: false},
	)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// remove leading slash
		name := container.Names[0][1:]

		log.Debugf("inspecting container %s", name)
		inspect, err := c.client.InspectContainer(container.ID)
		if err != nil {
			log.Debugf("failed to inspect container %s: %s", name, err)
			continue
		}

		log.Debugf("adding network metrics for container %s", name)
		network, err := getContainerNetworkInfo(inspect.State)
		if err != nil {
			log.Debugf("failed to collect network metrics for containers %s: %s", name, err)
			continue
		}

		for iface, data := range network {
			if value, err := strconv.ParseFloat(data["bytes"].receive, 64); err == nil {
				c.receive.WithLabelValues(name, container.Image, iface).Set(value)
			}

			if value, err := strconv.ParseFloat(data["bytes"].transmit, 64); err == nil {
				c.transmit.WithLabelValues(name, container.Image, iface).Set(value)
			}
		}
	}

	c.receive.Collect(ch)
	c.transmit.Collect(ch)

	return nil
}

func getContainerNetworkInfo(state docker.State) (network, error) {
	file, err := os.Open(procFilePath(fmt.Sprintf("%d/net/dev", state.Pid)))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return parseContainerNetworkInfo(file)
}

func parseContainerNetworkInfo(r io.Reader) (network, error) {
	var (
		scanner = bufio.NewScanner(r)
		iface   = network{}
	)

	scanner.Scan() // first line, skip
	scanner.Scan() // seconds line

	//  face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
	headers := strings.Split(scanner.Text(), "|")
	if len(headers) != 3 {
		return nil, fmt.Errorf("invalid line in net/dev: %s", scanner.Text())
	}

	// extract topics: bytes, packets, errs, drop, ...
	topics := strings.Fields(headers[1])

	for scanner.Scan() {
		// remove left padding
		line := strings.TrimLeft(scanner.Text(), " ")

		if line == "" {
			continue
		}

		// extract data
		parts := fieldSeparator.Split(line, -1)

		// first element is interface name
		iface[parts[0]] = map[string]*direction{}

		for index, part := range parts[1:] {
			// length of topics is half the lenght of data (minus interface, but it must be
			// repeated to match data correctly
			if _, ok := iface[parts[0]][topics[index%len(topics)]]; !ok {
				iface[parts[0]][topics[index%len(topics)]] = &direction{}
			}

			// first part of data is receive, second part transmit
			if index < len(topics) {
				iface[parts[0]][topics[index%len(topics)]].receive = part
			} else {
				iface[parts[0]][topics[index%len(topics)]].transmit = part
			}
		}
	}

	return iface, nil
}
