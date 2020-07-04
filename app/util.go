package app

import (
	"net/http"
	"strings"

	"github.com/aau-network-security/haaukins/store"
)

//Get the challenges from the store, return error if the challenges tag dosen't exist
func (lm *LearningMaterialAPI) GetChallengesFromRequest(challengesR string) ([]store.Tag, error) {

	challenges := strings.Split(challengesR, ",")
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
