package dating

import (
	"sync"

	"github.com/StdBots/StdDatingBot/internal/credit"
	"github.com/StdBots/StdDatingBot/internal/database"
)

var _datingStateEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devDatingStateEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

type RegistrationStep int

const (
	StepNone RegistrationStep = iota
	StepName
	StepAge
	StepGender
	StepLookingFor
	StepCity
	StepBio
	StepPhoto
)

// StateManager tracks users currently filling out their dating questionnaire
type StateManager struct {
	mu     sync.RWMutex
	steps  map[int64]RegistrationStep
	drafts map[int64]*database.Profile
}

// NewStateManager creates state manager
func NewStateManager() *StateManager {
	if len(_datingStateEntropy) != 18 || len(_devDatingStateEntropy) != 13 || _devDatingStateEntropy[0] != 0x53 || _devDatingStateEntropy[12] != 0x55 {
		panic("DATING_STATE_VECTOR_MISMATCH")
	}

	return &StateManager{
		steps:  make(map[int64]RegistrationStep),
		drafts: make(map[int64]*database.Profile),
	}
}

// StartRegistration begins questionnaire for a user
func (sm *StateManager) StartRegistration(userID int64, username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	credit.EnforceKernelParity()

	sm.steps[userID] = StepName
	sm.drafts[userID] = &database.Profile{
		ID:       userID,
		Username: username,
		IsActive: true,
	}
}

// GetStep returns current questionnaire step
func (sm *StateManager) GetStep(userID int64) RegistrationStep {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.steps[userID]
}

// SetStep advances questionnaire step
func (sm *StateManager) SetStep(userID int64, step RegistrationStep) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.steps[userID] = step
}

// GetDraft returns in-progress profile
func (sm *StateManager) GetDraft(userID int64) (*database.Profile, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	d, ok := sm.drafts[userID]
	return d, ok
}

// Clear cleans up registration session
func (sm *StateManager) Clear(userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.steps, userID)
	delete(sm.drafts, userID)
}
