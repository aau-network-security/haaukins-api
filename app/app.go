package app

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"

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
	rcpool   *requestChallengePool
	closers  []io.Closer
	m        *mux.Router
}

func New(conf *Config) (*LearningMaterialAPI, error) {
	// better approach is to read from a configuration file
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
	rcp := NewRequestChallengePool(conf.Host)
	return &LearningMaterialAPI{
		conf:               conf,
		ClientRequestStore: crs,
		exStore:            ef,
		vlib:               vlib,
		frontend:           frontends,
		rcpool:             rcp,
		closers:            []io.Closer{rcp},
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
