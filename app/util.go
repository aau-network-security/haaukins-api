package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/aau-network-security/haaukins/virtual/docker"

	"github.com/dgrijalva/jwt-go"

	"github.com/aau-network-security/haaukins/store"
)

//Gracefully shut down function
func (lm *LearningMaterialAPI) Close() error {
	var errs error
	var wg sync.WaitGroup

	for _, c := range lm.closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil && errs == nil {
				errs = err
			}
			wg.Done()
		}(c)
	}

	wg.Wait()

	if err := docker.DefaultLinkBridge.Close(); err != nil {
		return err
	}

	return errs
}

//Get the challenges from the store (haaukins), return error if the challenges tag dosen't exist
func (lm *LearningMaterialAPI) GetChallengesFromRequest(requestedChallenges string) ([]store.Tag, error) {

	challenges := strings.Split(requestedChallenges, ",")
	tags := make([]store.Tag, len(challenges))
	for i, s := range challenges {
		t := store.Tag(s)
		_, tagErr := lm.exStore.GetExercisesByTags(t)
		if tagErr != nil {
			return nil, tagErr
		}
		tags[i] = t
	}
	return tags, nil
}

//Create the token that will be used as a cookie
func (c *client) CreateToken(key string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		JWT_CLIENT_ID: c.id,
	})
	tokenStr, err := token.SignedString([]byte(key))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

//Get the token from the cookie
func GetTokenFromCookie(token, key string) (string, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
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

//todo use only  a function to make the response
func ClientTooManyRequests(w http.ResponseWriter) {
	w.WriteHeader(http.StatusTooManyRequests)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(tooManyRequestsHTMLTemplate))
	return
}

func TooManyRequests(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(tooManyRequestsHTMLTemplate))
	return
}

func ErrorResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(errorHTMLTemplate))
	return
}

func WaitingResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(waitingHTMLTemplate))
	return
}
