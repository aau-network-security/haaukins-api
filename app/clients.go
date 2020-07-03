package app

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const JWT_CLIENT_ID = "CLIENT_ID"

var (
	UnknownIDErr          = errors.New("unknown Client ID")
	ErrInvalidTokenFormat = errors.New("invalid token format")
	ErrChallengeNotFound  = errors.New("client challenge not found")
)

type ClientRequestStore interface {
	NewClient(string) *Client
	GetClient(string) (*Client, error)
}

type clientRequestStore struct {
	m        sync.RWMutex
	clientsR map[string]*Client //map with the client ip
}

func NewClientRequestStore() ClientRequestStore {
	crs := &clientRequestStore{
		clientsR: map[string]*Client{},
	}
	return crs
}

func (c *clientRequestStore) GetClient(id string) (*Client, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	client, ok := c.clientsR[id]
	if !ok {
		return &Client{}, UnknownIDErr
	}
	return client, nil

}

func (c *clientRequestStore) NewClient(host string) *Client {
	c.m.Lock()
	defer c.m.Unlock()

	id := uuid.New().String()

	_, ok := c.clientsR[id] //get a new id if the previous one already exists
	if ok {
		id = uuid.New().String()
	}

	cl := &Client{
		username:     id,
		password:     id,
		host:         host,
		requestsMade: 0,
		challenges:   map[string]ClientChallenge{},
	}

	c.clientsR[id] = cl
	return cl
}

type Client struct {
	m            sync.RWMutex
	username     string
	password     string
	host         string
	requestsMade int
	challenges   map[string]ClientChallenge
}

type ClientChallenge struct {
	isReady    bool
	guacCookie string
	guacPort   string
}

func (c *Client) GetChallenge(chals string) (ClientChallenge, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	cc, ok := c.challenges[chals]
	if !ok {
		return ClientChallenge{}, ErrChallengeNotFound
	}
	return cc, nil
}

func (c *Client) CreateToken(key string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		JWT_CLIENT_ID: c.username,
	})
	tokenStr, err := token.SignedString([]byte(key))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

func GetTokenFromCookie(token, key string) (string, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(key), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return "", ErrInvalidTokenFormat
	}

	id, ok := claims[JWT_CLIENT_ID].(string)
	if !ok {
		return "", ErrInvalidTokenFormat
	}
	return id, nil
}
