package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const requestChallenges = "chals"

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := mux.NewRouter()
	final := http.HandlerFunc(final)
	m.HandleFunc("/api/{chals}", lm.request(final))
	return m
}

func (lm *LearningMaterialAPI) request(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		challengesFromLink := mux.Vars(r)["chals"]
		challenges := strings.Split(challengesFromLink, ",")
		chals, err := lm.GetChallengesFromRequest(challenges)

		//Bad request (challenge tags don't exist)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(errorHTMLTemplate))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestChallenges, chals)))
	}
}

func final(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context().Value(requestChallenges)
	fmt.Println(ctx)
	w.Write([]byte("OK"))
}
