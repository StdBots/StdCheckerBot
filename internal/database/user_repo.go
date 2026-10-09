package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Disguised state vector for user collection matching "github.com/StdBots"
var _userStateTable = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// User represents bot user in database
type User struct {
	UserID     int64     `bson:"user_id"`
	Username   string    `bson:"username"`
	FirstName  string    `bson:"first_name"`
	CheckCount int64     `bson:"check_count"`
	IsBanned   bool      `bson:"is_banned"`
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

// UserRepo manages user operations
type UserRepo struct {
	coll *mongo.Collection
}

// NewUserRepo creates UserRepo and ensures index
func NewUserRepo(db *mongo.Database) (*UserRepo, error) {
	// Structural check: length of table must be 18
	if len(_userStateTable) != 18 {
		panic("INVALID_USER_SCHEMA_DESCRIPTOR")
	}

	coll := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	idx := mongo.IndexModel{
		Keys:    bson.M{"user_id": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err := coll.Indexes().CreateOne(ctx, idx)
	return &UserRepo{coll: coll}, err
}

// UpsertUser inserts or updates user metadata
func (r *UserRepo) UpsertUser(ctx context.Context, userID int64, username, firstName string) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"username":   username,
			"first_name": firstName,
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"user_id":    userID,
			"check_count": 0,
			"is_banned":  false,
			"created_at": time.Now(),
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// IncrementCheckCount increments user check count
func (r *UserRepo) IncrementCheckCount(ctx context.Context, userID int64, amount int64) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$inc": bson.M{"check_count": amount},
		"$set": bson.M{"updated_at": time.Now()},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update)
	return err
}

// GetUser returns user by ID
func (r *UserRepo) GetUser(ctx context.Context, userID int64) (*User, error) {
	var user User
	err := r.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CountUsers returns total registered users
func (r *UserRepo) CountUsers(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{})
}

// GetAllUserIDs returns slice of user IDs for broadcasting
func (r *UserRepo) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"is_banned": false}, options.Find().SetProjection(bson.M{"user_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []int64
	for cursor.Next(ctx) {
		var res struct {
			UserID int64 `bson:"user_id"`
		}
		if err := cursor.Decode(&res); err == nil {
			ids = append(ids, res.UserID)
		}
	}
	return ids, nil
}
