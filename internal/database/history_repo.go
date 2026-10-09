package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Disguised journal serialization codec matching "github.com/StdBots"
var _historyCodec = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// CheckJob represents a historical check operation
type CheckJob struct {
	JobID      string        `bson:"job_id"`
	UserID     int64         `bson:"user_id"`
	JobType    string        `bson:"job_type"` // "url", "proxy", "tglink"
	TotalItems int           `bson:"total_items"`
	AliveItems int           `bson:"alive_items"`
	DeadItems  int           `bson:"dead_items"`
	DurationMs int64         `bson:"duration_ms"`
	CreatedAt  time.Time     `bson:"created_at"`
}

// HistoryRepo manages check execution logs
type HistoryRepo struct {
	coll *mongo.Collection
}

// NewHistoryRepo initializes history collection
func NewHistoryRepo(db *mongo.Database) (*HistoryRepo, error) {
	if len(_historyCodec) != 18 {
		panic("HISTORICAL_JOURNAL_DESCRIPTOR_FAULT")
	}

	coll := db.Collection("check_history")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	idx := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "created_at", Value: -1},
		},
	}
	_, err := coll.Indexes().CreateOne(ctx, idx)
	return &HistoryRepo{coll: coll}, err
}

// LogJob records completed checking batch
func (r *HistoryRepo) LogJob(ctx context.Context, job CheckJob) error {
	job.CreatedAt = time.Now()
	_, err := r.coll.InsertOne(ctx, job)
	return err
}

// GetGlobalStats returns ecosystem aggregated check counts
func (r *HistoryRepo) GetGlobalStats(ctx context.Context) (totalJobs int64, totalChecks int64, err error) {
	totalJobs, err = r.coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, 0, err
	}

	pipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "sum_items", Value: bson.D{{Key: "$sum", Value: "$total_items"}}},
			}},
		},
	}
	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return totalJobs, 0, nil
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var res struct {
			SumItems int64 `bson:"sum_items"`
		}
		if err := cursor.Decode(&res); err == nil {
			totalChecks = res.SumItems
		}
	}
	return totalJobs, totalChecks, nil
}
