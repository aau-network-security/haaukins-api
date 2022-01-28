package app

import (
	"errors"
	"io/ioutil"

	"github.com/google/uuid"

	"github.com/aau-network-security/haaukins/daemon"
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
	TLS CertificateConfig `yaml:"tls,omitempty"`
	//ExercisesFile       string                           `yaml:"exercises-file,omitempty"`
	ExerciseService     daemon.ServiceConfig             `yaml:"exercise-service"`
	OvaDir              string                           `yaml:"ova-dir"`
	API                 APIConfig                        `yaml:"api"`
	SecretChallengeAuth Auth                             `yaml:"api-creds"`
	DockerRepositories  []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
}

type CertificateConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certfile"`
	CertKey  string `yaml:"certkey"`
	CAFile   string `yaml:"cafile"`
}

type Auth struct {
	EnableSecretAuth bool   `yaml:"enable-secret-auth"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
}

type APIConfig struct {
	SignKey string `yaml:"sign-key"`
	Admin   Auth   `yaml:"admin"`
	Captcha struct {
		Enabled   bool   `yaml:"enabled"`
		SiteKey   string `yaml:"site-key"`
		SecretKey string `yaml:"secret-key"`
	} `yaml:"captcha"`
	TotalMaxRequest  int `yaml:"total-max-requests"`
	ClientMaxRequest int `yaml:"client-max-requests"`
	FrontEnd         struct {
		Image  string `yaml:"image"`
		Memory uint   `yaml:"memory"`
	} `yaml:"frontend"`
	StoreFile string `yaml:"store-file"`
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

	if c.Host == "" {
		c.Host = "localhost"
	}

	if c.Port.InSecure == 0 {
		c.Port.InSecure = 80
	}

	if c.Port.Secure == 0 {
		c.Port.Secure = 443
	}

	if c.TLS.CertFile == "" || c.TLS.CertKey == "" {
		c.TLS.Enabled = false
	}

	random := uuid.New().String()

	if c.API.SignKey == "" {
		c.API.SignKey = random
	}

	if c.API.Admin.Username == "" {
		c.API.Admin.Username = random
	}

	if c.API.Admin.Password == "" {
		c.API.Admin.Password = random
	}

	if c.OvaDir == "" {
		return nil, errors.New("ova directory is necessary")
	}

	return &c, nil
}
