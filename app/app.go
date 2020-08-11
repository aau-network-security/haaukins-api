package app

import (
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
)

type LearningMaterialAPI struct {
	conf *Config
	ClientRequestStore
	exStore  store.ExerciseStore
	vlib     vbox.Library
	frontend []store.InstanceConfig
	closers  []io.Closer
}

func New(conf *Config) (*LearningMaterialAPI, error) {

	vlib := vbox.NewLibrary(conf.OvaDir)
	frontends := []store.InstanceConfig{{
		Image:    conf.API.FrontEnd.Image,
		MemoryMB: conf.API.FrontEnd.Memory,
	}}
	ef, err := store.NewExerciseFile(conf.ExercisesFile)
	if err != nil {
		return nil, err
	}
	crs := NewClientRequestStore()

	return &LearningMaterialAPI{
		conf:               conf,
		ClientRequestStore: crs,
		exStore:            ef,
		vlib:               vlib,
		frontend:           frontends,
		closers:            []io.Closer{crs},
	}, nil
}

func (lm *LearningMaterialAPI) Run() {
	log.Info().Msg("API ready to get requests")
	if lm.conf.Certs.Enabled {
		if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", lm.conf.Port.Secure), lm.conf.Certs.CertFile, lm.conf.Certs.CertKey, lm.Handler()); err != nil {
			log.Warn().Msgf("Serving error: %s", err)
		}
		return
	}
	if err := http.ListenAndServe(fmt.Sprintf(":%d", lm.conf.Port.InSecure), lm.Handler()); err != nil {
		log.Warn().Msgf("Serving error: %s", err)
	}
}
