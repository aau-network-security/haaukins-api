package app

import (
	"sync"
)

type ClientRequestStore interface {
	GetClientRequests() []string
	SaveClient()
	GetClient()
}

type clientRequestStore struct {
	m        sync.RWMutex
	clientsR map[string]string //map with the client ip
}

func (c *clientRequestStore) GetClientRequests() []string {
	panic("implement me")
}

func (c *clientRequestStore) SaveClient() {
	panic("implement me")
}

func (c *clientRequestStore) GetClient() {
	panic("implement me")
}

func NewClientRequestStore() ClientRequestStore {
	crs := &clientRequestStore{
		clientsR: map[string]string{},
	}
	return crs
}

//
//func (crs *ClientRequestStore) NewClientRequest(host string) *ClientRequest {
//
//	cl := &ClientRequest{
//		username:     uuid.New().String(),
//		password:     uuid.New().String(),
//		cookies:      map[string]string{},
//		host:         host,
//		ports:        map[string]uint{},
//		requestsMade: 0,
//	}
//
//	crs.clientsR[host] = cl
//	return cl
//}
//
//type ClientRequest struct {
//	username     string
//	password     string
//	cookies      map[string]string //map cookie with challenges tag
//	host         string
//	ports        map[string]uint //map guacamole port with challenges tag
//	requestsMade int
//}
