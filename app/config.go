package app

import (
	"io/ioutil"

	"github.com/aau-network-security/haaukins/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Host string `yaml:"host,omitempty"`
	Port struct {
		Secure   uint `yaml:"secure,omitempty"`
		InSecure uint `yaml:"insecure,omitempty"`
	} `yaml:"port"`
	Certs              CertificateConfig                `yaml:"tls,omitempty"`
	ExercisesFile      string                           `yaml:"exercises-file,omitempty"`
	OvaDir             string                           `yaml:"ova-dir"`
	API                APIConfig                        `yaml:"api"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
}

type CertificateConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certfile"`
	CertKey  string `yaml:"certkey"`
	CAFile   string `yaml:"cafile"`
}

type APIConfig struct {
	SignKey string `yaml:"sign-key"`
	Admin   struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	TotalMaxRequest  int `yaml:"total-max-requests"`
	ClientMaxRequest int `yaml:"client-max-requests"`
	FrontEnd         struct {
		Image  string `yaml:"image"`
		Memory uint   `yaml:"memory"`
	} `yaml:"frontend"`
}

func NewConfigFromFile(path string) (*Config, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		return nil, err
	}

	for _, repo := range c.DockerRepositories {
		docker.Registries[repo.ServerAddress] = repo
	}

	//todo manage the error in the config file
	return &c, nil
}
