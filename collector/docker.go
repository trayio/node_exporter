package collector

import docker "github.com/fsouza/go-dockerclient"

func NewDockerClient(url string) (*docker.Client, error) {
	if url == "" {
		url = "unix:///var/run/docker.sock"
	}

	return docker.NewClient(url)
}
