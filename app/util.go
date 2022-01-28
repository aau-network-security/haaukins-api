package app

import (
	"context"
	"encoding/csv"
	"fmt"
	proto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	"io"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"github.com/rs/zerolog/log"

	"github.com/aau-network-security/haaukins/virtual/docker"

	"github.com/dgrijalva/jwt-go"

	"github.com/aau-network-security/haaukins/store"
	"github.com/golang/protobuf/jsonpb"
	bproto "github.com/golang/protobuf/proto"
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
func (lm *LearningMaterialAPI) GetChallengesFromRequest(requestedChallenges string) ([]store.Tag, []string, error) {

	challenges := strings.Split(requestedChallenges, ",")
	tags := make([]store.Tag, len(challenges))
	ctx := context.TODO()
	for i, s := range challenges {
		t := store.Tag(s)
		_, tagErr := lm.exClient.GetExerciseByTags(ctx, &proto.GetExerciseByTagsRequest{Tag: []string{s}})
		if tagErr != nil {
			return nil, nil, tagErr
		}
		tags[i] = t
	}
	return tags, challenges, nil
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

func writeToCSVFile(w *csv.Writer, info []string) {
	if err := w.Write(info); err != nil {
		log.Error().Msgf("Error writing the CSV File %v", err)
		return
	}
	w.Flush()
}

func notFoundPage(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles(
		"resources/private/base.tmpl.html",
		"resources/private/404.tmpl.html",
	)
	if err != nil {
		log.Error().Msgf("error index tmpl: %s", err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNotFound)
	if err := tmpl.Execute(w, nil); err != nil {
		http.NotFound(w, r)
		log.Error().Msgf("template err index: %s", err.Error())
	}
}

type returnError struct {
	Content         string
	Toomanyrequests bool
}

func errorPage(w http.ResponseWriter, r *http.Request, statusCode int, error returnError) {

	tmpl, err := template.ParseFiles(
		"resources/private/base.tmpl.html",
		"resources/private/error.tmpl.html",
	)
	if err != nil {
		log.Error().Msgf("error index tmpl: %s", err.Error())
		w.WriteHeader(statusCode)
		return
	}

	w.WriteHeader(statusCode)
	if err := tmpl.Execute(w, error); err != nil {
		http.NotFound(w, r)
		log.Error().Msgf("template err index: %s", err.Error())
	}
}

func WaitingResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(waitingHTMLTemplate))
	return
}

func protobufToJson(message bproto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
	}

	return marshaler.MarshalToString(message)
}
