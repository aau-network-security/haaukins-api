package app

import (
	"context"
	"encoding/json"
	"fmt"
	proto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"github.com/aau-network-security/haaukins/store"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var (
	newline = []byte{'\n'}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Message struct {
	Message string      `json:"msg"`
	Values  interface{} `json:"values"`
}

type Category struct {
	Name       string      `json:"name"`
	Tag        string      `json:"tag"`
	Challenges []Challenge `json:"challenges"`
}

type Challenge struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type FrontendClient struct {
	conn *websocket.Conn
	send chan []byte
}

//Send the chalenges to the API frontend in order to let the users select which one run on their environment
func (lm *LearningMaterialAPI) handleFrontendChallengesRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		client := &FrontendClient{conn: conn, send: make(chan []byte, 256)}

		var categories []Category

		//Get che Categories
		cats, err := lm.getChallengeCategories()
		if err != nil {
			log.Println(err)
		}
		for _, c := range cats {
			categories = append(categories, Category{
				Name:       c.Name,
				Tag:        strings.ToLower(string(c.Tag)),
				Challenges: []Challenge{},
			})
		}

		//loop through the exercises
		ctx := context.TODO()
		response, err := lm.exClient.GetExercises(ctx, &proto.Empty{})
		if err != nil {
			log.Println(fmt.Errorf("[exercise-service] Error getting exercises: %v", err))
		}

		for _, e := range response.Exercises {
			if e.Secret {
				continue
			}
			exercise, err := protobufToJson(e)
			if err != nil {
				log.Println("Error converting protobuffer to JSON: %v", err)
			}
			eStruct := store.Exercise{}
			json.Unmarshal([]byte(exercise), &eStruct)
			chal := Challenge{
				Name: e.Name,
				Tag:  e.Tag,
			}

			var category string
			for _, i := range eStruct.Instance {
				if len(i.Flags) != 0 {
					category = i.Flags[0].Category
					if len(i.Flags) > 1 {
						chal.Name += " ("
						for _, f := range i.Flags {
							chal.Name += f.Name + ", "
						}
						chal.Name = strings.TrimRight(chal.Name, ", ") + ")"
					}
				}
			}


			for i, rc := range categories {
				if rc.Name == category{
					categories[i].Challenges = append(categories[i].Challenges, chal)
				}
			}

		}

		msg := Message{
			Message: "challenges_categories",
			Values:  categories,
		}

		rawMsg, _ := json.Marshal(msg)

		client.send <- rawMsg

		go client.writePump()

	}
}

//Send a message to frontend. It uses s ticker in order to close the connection after the data is sent
func (c *FrontendClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//Get challenges categories
func (lm *LearningMaterialAPI) getChallengeCategories() ([]store.Category, error) {
	challengeCats := []store.Category{}
	ctx := context.TODO()
	response, err := lm.exClient.GetCategories(ctx, &proto.Empty{})
	if err != nil {
		return nil, fmt.Errorf("[exercise-service] Error getting categories")
	}
	for _, c := range response.Categories {
		category, err := protobufToJson(c)
		if err != nil {
			return nil, err
		}
		catStruct := store.Category{}
		json.Unmarshal([]byte(category), &catStruct)
		challengeCats = append(challengeCats, catStruct)
	}
	return challengeCats, nil
}
