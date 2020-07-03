package app

import (
	"github.com/aau-network-security/haaukins/store"
)

//Get the challenges from the store, return error if the challenges tag dosen't exist
func (lm *LearningMaterialAPI) GetChallengesFromRequest(challenges []string) ([]store.Tag, error) {

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
