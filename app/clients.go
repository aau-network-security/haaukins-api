package app

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

const JWT_CLIENT_ID = "CLIENT_ID"

var (
	UnknownIDErr          = errors.New("unknown Client ID")
	ErrInvalidTokenFormat = errors.New("invalid token format")
	ErrChallengeNotFound  = errors.New("client challenge not found")
)

type ClientRequestStore interface {
	NewClient(string) Client
	GetClient(string) (Client, error)
}

type clientRequestStore struct {
	m        sync.RWMutex
	clientsR map[string]*client //map with the client ip
}

func NewClientRequestStore() ClientRequestStore {
	crs := &clientRequestStore{
		clientsR: map[string]*client{},
	}
	return crs
}

func (c *clientRequestStore) GetClient(id string) (Client, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	client, ok := c.clientsR[id]
	if !ok {
		return nil, UnknownIDErr
	}
	return client, nil

}

func (c *clientRequestStore) NewClient(host string) Client {
	c.m.Lock()
	defer c.m.Unlock()

	id := uuid.New().String()

	_, ok := c.clientsR[id] //get a new id if the previous one already exists
	if ok {
		id = uuid.New().String()
	}

	cl := &client{
		id:           id,
		host:         host,
		requestsMade: 0,
		challenges:   map[string]*ClientChallenge{},
	}

	c.clientsR[id] = cl
	return cl
}

type Client interface {
	GetChallenge(string) (*ClientChallenge, error)
	NewClientChallenge(string) *ClientChallenge
	CreateToken(key string) (string, error)
	ID() string
	RequestMade() int
	AddRequest()
}

type client struct {
	m            sync.RWMutex
	id           string
	host         string
	requestsMade int
	challenges   map[string]*ClientChallenge
}

func (c *client) GetChallenge(chals string) (*ClientChallenge, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	cc, ok := c.challenges[chals]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	return cc, nil
}

func (c *client) NewClientChallenge(chals string) *ClientChallenge {
	c.m.Lock()
	defer c.m.Unlock()

	cc := &ClientChallenge{
		isReady: false,
		err:     make(chan error, 0),
	}

	c.challenges[chals] = cc

	return cc
}

type ClientChallenge struct {
	isReady    bool
	err        chan error
	guacCookie string
	guacPort   uint
}

func (cc *ClientChallenge) NewError(e error) {
	cc.err <- e
}

func (c *client) ID() string {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.id
}

func (c *client) RequestMade() int {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.requestsMade
}

func (c *client) AddRequest() {
	c.m.Lock()
	defer c.m.Unlock()
	c.requestsMade += 1
	return
}
