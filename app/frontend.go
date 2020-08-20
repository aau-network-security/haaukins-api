package app

import (
	"encoding/json"
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
	space   = []byte{' '}
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

func (lm *LearningMaterialAPI) handleFrontendChallengesRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		client := &FrontendClient{conn: conn, send: make(chan []byte, 256)}

		var categories []Category
		for _, c := range lm.getChallengeCategories() {
			tag := strings.Join(strings.Fields(c), "")
			categories = append(categories, Category{
				Name:       c,
				Tag:        strings.ToLower(tag),
				Challenges: []Challenge{},
			})
		}

		for _, exercise := range lm.exStore.ListExercises() {
			chal := Challenge{
				Name: exercise.Name,
				Tag:  string(exercise.Tags[0]),
			}

			flags := exercise.Flags()
			if len(flags) > 1 {
				chal.Name += " ("
				for _, f := range flags {
					chal.Name += f.Name + ", "
				}
				chal.Name = strings.TrimRight(chal.Name, ", ") + ")"
			}

			for i, rc := range categories {
				if rc.Name == flags[0].Category {
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

func (lm *LearningMaterialAPI) getChallengeCategories() []string {
	keys := make(map[string]bool)
	challengeCats := []string{}
	for _, challenge := range lm.exStore.ListExercises() {
		for _, f := range challenge.Flags() {
			if _, value := keys[f.Category]; !value {
				keys[f.Category] = true
				challengeCats = append(challengeCats, f.Category)
			}
		}
	}
	return challengeCats
}
