package app

import (
	"errors"
	"net/http"
	"sync"
)

var (
	UnknownRequestChallenge = errors.New("unable to find RequestChallege by that id")
)

type requestChallengePool struct {
	m                sync.RWMutex
	host             string
	requestChallenge map[string]Environment
	handlers         map[string]http.Handler
}

func NewRequestChallengePool(host string) *requestChallengePool {
	return &requestChallengePool{
		host:             host,
		requestChallenge: map[string]Environment{},
		handlers:         map[string]http.Handler{}, //maybe need it for the proxy handler
	}
}

func (rcp *requestChallengePool) AddRequestChallenge(env Environment) {

	rcp.m.Lock()
	defer rcp.m.Unlock()

	rcp.requestChallenge[env.ID()] = env
	//assing the proxy handler here in case
}

func (rcp *requestChallengePool) RemoveRequestChallenge(id string) error {

	rcp.m.Lock()
	defer rcp.m.Unlock()

	if _, ok := rcp.requestChallenge[id]; !ok {
		return UnknownRequestChallenge
	}
	delete(rcp.requestChallenge, id)
	//remove the proxy handler here in case

	return nil
}

func (rcp *requestChallengePool) GetRequestChallenge(id string) (Environment, error) {

	rcp.m.RLock()
	defer rcp.m.RUnlock()

	env, ok := rcp.requestChallenge[id]

	if !ok {
		return nil, UnknownRequestChallenge
	}

	return env, nil
}

func (rcp *requestChallengePool) GetALLRequestsChallenge() []Environment {

	envs := make([]Environment, len(rcp.requestChallenge))
	var i int

	rcp.m.RLock()
	defer rcp.m.RUnlock()

	for _, env := range rcp.requestChallenge {
		envs[i] = env
		i++
	}
	return envs
}

func (rcp *requestChallengePool) Close() error {
	var firstErr error
	for _, rc := range rcp.requestChallenge {
		if err := rc.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
