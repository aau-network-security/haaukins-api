package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	hlab "github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/rs/zerolog/log"
)

const environmentTimer = 45 * time.Minute

type environment struct {
	timer      *time.Timer
	challenges []store.Tag
	lab        hlab.Lab
	guacamole  guacamole.Guacamole
}

type Environment interface {
	GetChallenges() string
	GetTimer() *time.Timer
	Assign(Client, string) error
	Close() error //close the dockers and the vms
}

//Create a new environment (Haaukins Lab)
func (lm *LearningMaterialAPI) NewEnvironment(challenges []store.Tag) (Environment, error) {

	ctx := context.Background()
	exercises, _ := lm.exStore.GetExercisesByTags(challenges...)

	labConf := hlab.Config{
		Exercises: exercises,
		Frontends: lm.frontend,
	}

	lh := hlab.LabHost{
		Vlib: lm.vlib,
		Conf: labConf,
	}

	lab, err := lh.NewLab(ctx)
	if err != nil {
		log.Error().Msgf("Error while creating new lab %s", err.Error())
		return nil, err
	}

	if err := lab.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting lab %s", err.Error())
		return nil, err
	}

	env := &environment{
		timer:      time.NewTimer(environmentTimer),
		challenges: challenges,
		lab:        lab,
		guacamole:  lm.guacamole,
	}

	return env, nil
}

//Assign the environment to the client
func (e *environment) Assign(client Client, chals string) error {

	cr, err := client.GetClientRequest(chals)
	if err != nil {
		return errors.New("client request not found")
	}

	rdpPorts := e.lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return errors.New("RdpConfErr")
	}

	u := guacamole.GuacUser{
		Username: cr.ID(),
		Password: cr.ID(),
	}

	if err := e.guacamole.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	dockerHost := docker.NewHost()
	hostIp, err := dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", cr.ID(), num)

		log.Debug().Str("client", cr.ID()).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := e.guacamole.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: u.Username,
			Username: &u.Username,
			Password: &u.Password,
		}); err != nil {
			return err
		}
	}

	cr.env = e
	cr.isReady = true

	return nil
}

func (e *environment) GetChallenges() string {
	chals := make([]string, len(e.challenges))
	var i int
	for _, c := range e.challenges {
		chals[i] = string(c)
		i++
	}
	return strings.Join(chals, ",")
}

func (e *environment) GetTimer() *time.Timer {
	return e.timer
}

func (e *environment) Close() error {
	err := e.lab.Close()
	return err
}
