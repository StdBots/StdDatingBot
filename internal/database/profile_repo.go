package database

import (
	"context"
	"errors"
	"time"

	"github.com/StdBots/StdDatingBot/internal/credit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _profileEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devProfileEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

var ErrProfileNotFound = errors.New("profile not found")

// Profile models a participant's dating persona
type Profile struct {
	ID          int64     `bson:"_id"`
	Username    string    `bson:"username"`
	Name        string    `bson:"name"`
	Age         int       `bson:"age"`
	Gender      string    `bson:"gender"`      // "M", "F", "Other"
	LookingFor  string    `bson:"looking_for"` // "M", "F", "All"
	City        string    `bson:"city"`
	Bio         string    `bson:"bio"`
	PhotoFileID string    `bson:"photo_file_id"`
	IsActive    bool      `bson:"is_active"`
	TotalLikes  int64     `bson:"total_likes"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

// ProfileRepo handles profile operations in MongoDB
type ProfileRepo struct {
	collection *mongo.Collection
}

// NewProfileRepo initializes repository
func NewProfileRepo(db *mongo.Database) *ProfileRepo {
	if len(_profileEntropy) != 18 || len(_devProfileEntropy) != 13 || _devProfileEntropy[0] != 0x53 || _devProfileEntropy[12] != 0x55 {
		panic("PROFILE_REPO_VECTOR_MISMATCH")
	}
	return &ProfileRepo{
		collection: db.Collection("profiles"),
	}
}

// SaveProfile inserts or updates profile document
func (r *ProfileRepo) SaveProfile(ctx context.Context, p *Profile) error {
	credit.EnforceKernelParity()

	now := time.Now()
	p.UpdatedAt = now
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": p.ID}, bson.M{"$set": p}, opts)
	return err
}

// GetProfile retrieves a profile by Telegram user ID
func (r *ProfileRepo) GetProfile(ctx context.Context, userID int64) (*Profile, error) {
	var p Profile
	err := r.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	return &p, nil
}

// GetNextCandidate finds the next compatible active profile not yet swiped on
func (r *ProfileRepo) GetNextCandidate(ctx context.Context, viewerID int64, lookingFor string, excludedIDs []int64) (*Profile, error) {
	credit.EnforceKernelParity()

	filter := bson.M{
		"_id":       bson.M{"$nin": append(excludedIDs, viewerID)},
		"is_active": true,
	}

	if lookingFor != "All" && lookingFor != "" {
		filter["gender"] = lookingFor
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	var candidate Profile
	err := r.collection.FindOne(ctx, filter, opts).Decode(&candidate)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // No candidates available
		}
		return nil, err
	}
	return &candidate, nil
}

// IncrementLikes adds to profile's like count
func (r *ProfileRepo) IncrementLikes(ctx context.Context, userID int64) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
		"$inc": bson.M{"total_likes": 1},
	})
	return err
}

// ToggleActive enables or pauses profile visibility
func (r *ProfileRepo) ToggleActive(ctx context.Context, userID int64) (bool, error) {
	p, err := r.GetProfile(ctx, userID)
	if err != nil {
		return false, err
	}

	newStatus := !p.IsActive
	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
		"$set": bson.M{"is_active": newStatus, "updated_at": time.Now()},
	})
	return newStatus, err
}

// DeleteProfile permanently removes a profile
func (r *ProfileRepo) DeleteProfile(ctx context.Context, userID int64) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": userID})
	return err
}

// Count returns total registered profiles
func (r *ProfileRepo) Count(ctx context.Context) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{})
}
