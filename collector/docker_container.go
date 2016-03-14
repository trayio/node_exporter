package collector

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	dockerContainerSubsystem = "docker"
)

type dockerContainerCollector struct {
	client     *docker.Client
	containers *prometheus.GaugeVec
}

func init() {
	Factories["docker_containers"] = NewDockerContainerCollector
}

func NewDockerContainerCollector() (Collector, error) {
	var labels = []string{"name", "image"}

	var client, err = NewDockerClient("")
	if err != nil {
		return nil, err
	}

	return &dockerContainerCollector{
		client: client,
		containers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: dockerContainerSubsystem,
				Name:      "containers",
				Help:      "Running Docker containers",
			},
			labels,
		),
	}, nil
}

func (c *dockerContainerCollector) Update(ch chan<- prometheus.Metric) error {
	c.containers.Reset()

	containers, err := c.client.ListContainers(
		docker.ListContainersOptions{All: false},
	)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// remove leading slash
		name := container.Names[0][1:]

		log.Debugf("adding container %s", container.ID)
		c.containers.WithLabelValues(name, container.Image).Set(1)
	}

	c.containers.Collect(ch)

	return nil
}
