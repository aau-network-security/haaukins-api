package app

import (
	"context"
	"fmt"
	proto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"io"
	"net/http"
	"os"

	"github.com/aau-network-security/haaukins/svcs/guacamole"

	"github.com/rs/zerolog/log"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
)

type LearningMaterialAPI struct {
	conf *Config
	ClientRequestStore
	captcha   Recaptcha
	exClient  proto.ExerciseStoreClient
	vlib      vbox.Library
	frontend  []store.InstanceConfig
	storeFile *os.File
	closers   []io.Closer
	guacamole guacamole.Guacamole
}

func New(conf *Config, isTest bool) (*LearningMaterialAPI, error) {

	vlib := vbox.NewLibrary(conf.OvaDir)
	frontends := []store.InstanceConfig{{
		Image:    conf.API.FrontEnd.Image,
		MemoryMB: conf.API.FrontEnd.Memory,
	}}

	exServiceConfig := store.ServiceConfig{
		Grpc:     conf.ExerciseService.Grpc,
		AuthKey:  conf.ExerciseService.AuthKey,
		SignKey:  conf.ExerciseService.SignKey,
		Enabled:  conf.ExerciseService.CertConfig.Enabled,
		CertFile: conf.ExerciseService.CertConfig.CertFile,
		CertKey:  conf.ExerciseService.CertConfig.CertKey,
		CAFile:   conf.ExerciseService.CertConfig.CAFile,
	}

	exServiceClient, err := store.NewExerciseClientConn(exServiceConfig)
	if err != nil {
		return nil, fmt.Errorf("[Exercise Service] Error creating gRPC connection to exercise service: %v", err)
	}

	crs := NewClientRequestStore()

	sf, err := os.OpenFile(conf.API.StoreFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	var guac guacamole.Guacamole
	if !isTest {
		ctx := context.Background()
		guac, err = guacamole.New(ctx, guacamole.Config{}, 0)
		if err != nil {
			log.Error().Msgf("Error while creating new guacamole %s", err.Error())
			return nil, err
		}

		if err := guac.Start(ctx); err != nil {
			log.Error().Msgf("Error while starting guacamole %s", err.Error())
			return nil, err
		}
	}

	return &LearningMaterialAPI{
		conf:               conf,
		ClientRequestStore: crs,
		captcha:            NewRecaptcha(conf.API.Captcha.SecretKey),
		exClient:           exServiceClient,
		vlib:               vlib,
		frontend:           frontends,
		storeFile:          sf,
		closers:            []io.Closer{crs, sf, guac},
		guacamole:          guac,
	}, nil
}

func (lm *LearningMaterialAPI) Run() {
	log.Info().Msg("API ready to get requests")
	if lm.conf.TLS.Enabled {
		log.Info().Msgf("API running in SECURE mode under port: %d", lm.conf.Port.Secure)
		if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", lm.conf.Port.Secure), lm.conf.TLS.CertFile, lm.conf.TLS.CertKey, lm.Handler()); err != nil {
			log.Warn().Msgf("Serving error: %s", err)
		}
		return
	}
	log.Info().Msgf("API running under port: %d", lm.conf.Port.InSecure)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", lm.conf.Port.InSecure), lm.Handler()); err != nil {
		log.Warn().Msgf("Serving error: %s", err)
	}
}
