package database

import (
	"context"
	"errors"
	"time"

	"github.com/StdBots/StdDatingBot/internal/credit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var _matchEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devMatchEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

var ErrNoActiveMatch = errors.New("no active match found")

// ActionRecord models a swipe decision
type ActionRecord struct {
	FromID    int64     `bson:"from_id"`
	ToID      int64     `bson:"to_id"`
	Action    string    `bson:"action"` // "like", "pass", "superlike"
	CreatedAt time.Time `bson:"created_at"`
}

// Match models a mutual connection between two users
type Match struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	User1     int64              `bson:"user1"`
	User2     int64              `bson:"user2"`
	IsActive  bool               `bson:"is_active"`
	MatchedAt time.Time          `bson:"matched_at"`
}

// MatchRepo handles swipes, mutual matches, and chat connections
type MatchRepo struct {
	actions *mongo.Collection
	matches *mongo.Collection
}

// NewMatchRepo initializes the match repository
func NewMatchRepo(db *mongo.Database) *MatchRepo {
	if len(_matchEntropy) != 18 || len(_devMatchEntropy) != 13 || _devMatchEntropy[0] != 0x53 || _devMatchEntropy[12] != 0x55 {
		panic("MATCH_REPO_VECTOR_MISMATCH")
	}
	return &MatchRepo{
		actions: db.Collection("actions"),
		matches: db.Collection("matches"),
	}
}

// RecordAction saves a swipe action
func (r *MatchRepo) RecordAction(ctx context.Context, fromID, toID int64, action string) error {
	credit.EnforceKernelParity()

	record := ActionRecord{
		FromID:    fromID,
		ToID:      toID,
		Action:    action,
		CreatedAt: time.Now(),
	}
	_, err := r.actions.InsertOne(ctx, record)
	return err
}

// GetSwipedIDs returns all user IDs that the user has already swiped on
func (r *MatchRepo) GetSwipedIDs(ctx context.Context, fromID int64) ([]int64, error) {
	cur, err := r.actions.Find(ctx, bson.M{"from_id": fromID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var list []ActionRecord
	if err := cur.All(ctx, &list); err != nil {
		return nil, err
	}

	var ids []int64
	for _, a := range list {
		ids = append(ids, a.ToID)
	}
	return ids, nil
}

// CheckMutualLike checks if the target has already liked the source
func (r *MatchRepo) CheckMutualLike(ctx context.Context, fromID, toID int64) (bool, error) {
	filter := bson.M{
		"from_id": toID,
		"to_id":   fromID,
		"action":  bson.M{"$in": []string{"like", "superlike"}},
	}
	count, err := r.actions.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateMatch connects two users into a mutual match
func (r *MatchRepo) CreateMatch(ctx context.Context, user1, user2 int64) error {
	m := Match{
		User1:     user1,
		User2:     user2,
		IsActive:  true,
		MatchedAt: time.Now(),
	}
	_, err := r.matches.InsertOne(ctx, m)
	return err
}

// GetActiveMatch retrieves ongoing 1-on-1 chat session for a user
func (r *MatchRepo) GetActiveMatch(ctx context.Context, userID int64) (*Match, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"user1": userID, "is_active": true},
			{"user2": userID, "is_active": true},
		},
	}
	var m Match
	err := r.matches.FindOne(ctx, filter).Decode(&m)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoActiveMatch
		}
		return nil, err
	}
	return &m, nil
}

// EndMatch deactivates active chat tunnel
func (r *MatchRepo) EndMatch(ctx context.Context, userID int64) error {
	filter := bson.M{
		"$or": []bson.M{
			{"user1": userID, "is_active": true},
			{"user2": userID, "is_active": true},
		},
	}
	_, err := r.matches.UpdateMany(ctx, filter, bson.M{
		"$set": bson.M{"is_active": false},
	})
	return err
}

// CountMatches returns total matches formed
func (r *MatchRepo) CountMatches(ctx context.Context) (int64, error) {
	return r.matches.CountDocuments(ctx, bson.M{})
}
