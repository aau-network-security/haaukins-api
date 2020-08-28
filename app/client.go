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
	GetAllClients() []Client
	GetAllRequests() []*ClientRequest
	Close() error //To Shut down gracefully
}

type clientRequestStore struct {
	m        sync.RWMutex
	clientsR map[string]*client
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

func (c *clientRequestStore) GetAllClients() []Client {
	c.m.RLock()
	defer c.m.RUnlock()

	clients := make([]Client, len(c.clientsR))
	var i int
	for _, c := range c.clientsR {
		clients[i] = c
		i++
	}
	return clients
}

func (c *clientRequestStore) GetAllRequests() []*ClientRequest {
	c.m.RLock()
	defer c.m.RUnlock()

	clients := c.GetAllClients()
	var cr []*ClientRequest
	for _, client := range clients {
		for _, r := range client.GetAllClientRequests() {
			cr = append(cr, r)
		}
	}
	return cr
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
		id:       id,
		host:     host,
		requests: map[string]*ClientRequest{},
	}

	c.clientsR[id] = cl
	return cl
}

func (c *clientRequestStore) Close() error {
	c.m.RLock()
	defer c.m.RUnlock()

	var firstErr error
	for _, cr := range c.clientsR {
		for _, ce := range cr.requests {
			if err := ce.env.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

type Client interface {
	GetClientRequest(string) (*ClientRequest, error)
	GetAllClientRequests() []*ClientRequest
	NewClientRequest(string) *ClientRequest
	RemoveClientRequest(string)
	CreateToken(key string) (string, error)
	ID() string
	Host() string
	RequestMade() int
}

type client struct {
	m        sync.RWMutex
	id       string
	host     string
	requests map[string]*ClientRequest //map with the challengeTags
}

func (c *client) GetClientRequest(chals string) (*ClientRequest, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	cc, ok := c.requests[chals]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	return cc, nil
}

func (c *client) GetAllClientRequests() []*ClientRequest {
	c.m.RLock()
	defer c.m.RUnlock()

	requests := make([]*ClientRequest, len(c.requests))
	var i int
	for _, r := range c.requests {
		requests[i] = r
		i++
	}
	return requests

}

func (c *client) NewClientRequest(chals string) *ClientRequest {
	c.m.Lock()
	defer c.m.Unlock()

	cc := &ClientRequest{
		isReady: false,
		err:     make(chan error, 0),
	}

	c.requests[chals] = cc

	return cc
}

func (c *client) RemoveClientRequest(chals string) {
	c.m.Lock()
	defer c.m.Unlock()
	delete(c.requests, chals)
}

type ClientRequest struct {
	isReady    bool
	err        chan error
	env        Environment
	guacCookie string
	guacPort   uint
}

func (cr *ClientRequest) NewError(e error) {
	cr.err <- e
}

func (c *client) ID() string {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.id
}

func (c *client) Host() string {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.host
}

func (c *client) RequestMade() int {
	c.m.RLock()
	defer c.m.RUnlock()
	return len(c.requests)
}
