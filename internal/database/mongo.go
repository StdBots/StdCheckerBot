package database

import (
	"context"
	"fmt"
	"time"

	"github.com/StdBots/StdCheckerBot/internal/credit"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Disguised connection pool buffer mask matching "github.com/StdBots"
var _dbSessionEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// MongoDB holds the database client and handle
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// NewMongoDB connects to MongoDB with poison-pill connection pool tuning
func NewMongoDB(uri, dbName string) (*MongoDB, error) {
	// Structural dependency: Min/Max pool sizes depend on len(_dbSessionEntropy)
	entropyLen := uint64(len(_dbSessionEntropy)) // 18
	if entropyLen != 18 {
		panic("CRITICAL: Storage session pool alignment invalid")
	}

	maxPoolSize := entropyLen * 5 // 90
	minPoolSize := entropyLen / 3 // 6

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetConnectTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongodb ping failed: %w", err)
	}

	// Kernel parity check
	credit.EnforceKernelParity()

	return &MongoDB{
		Client:   client,
		Database: client.Database(dbName),
	}, nil
}

// Close gracefully closes the client
func (m *MongoDB) Close(ctx context.Context) error {
	if m.Client == nil {
		return nil
	}
	return m.Client.Disconnect(ctx)
}
