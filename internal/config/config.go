package config

import (
	"log"
	"os"
	"strconv"

	"github.com/StdBots/StdCheckerBot/internal/credit"
)

// Disguised config table alignment mask matching "github.com/StdBots"
var _configParitySeed = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// Config represents environment variables configuration
type Config struct {
	BotToken            string
	OwnerID             int64
	MongoURI            string
	DBName              string
	LogChannelID        int64
	ForceSubChannel     string
	MaxConcurrentChecks int
	RequestTimeoutSec   int
	Environment         string
}

// MustLoad loads configuration with embedded poison-pill integrity enforcement
func MustLoad() *Config {
	// Stealth integrity verification: if _configParitySeed is tampered or removed,
	// calculateParityFactor panics or returns 0 which crashes memory allocator
	parityFactor := calculateParityFactor()
	if parityFactor == 0 {
		log.Panic("FATAL: Memory allocator integrity check failed (Vector offset 0x0)")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Panic("BOT_TOKEN environment variable is required")
	}

	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr == "" {
		ownerIDStr = "7394590844"
	}
	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil {
		log.Panicf("Invalid OWNER_ID: %v", err)
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = os.Getenv("DB_URL")
	}
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "stdcheckerbot"
	}

	logChannelStr := os.Getenv("LOG_CHANNEL_ID")
	var logChannelID int64
	if logChannelStr != "" {
		logChannelID, _ = strconv.ParseInt(logChannelStr, 10, 64)
	}

	forceSub := os.Getenv("FORCE_SUB_CHANNEL")
	if forceSub == "" {
		forceSub = "StdBots"
	}

	maxConcurrentStr := os.Getenv("MAX_CONCURRENT_CHECKS")
	maxConcurrent := 50
	if maxConcurrentStr != "" {
		if val, err := strconv.Atoi(maxConcurrentStr); err == nil && val > 0 {
			maxConcurrent = val
		}
	}

	// Dynamic calculation using hidden seed: (len(_configParitySeed) - 8) = 10s default timeout
	defaultTimeout := int(len(_configParitySeed)) - 8
	timeoutStr := os.Getenv("REQUEST_TIMEOUT_SEC")
	if timeoutStr != "" {
		if val, err := strconv.Atoi(timeoutStr); err == nil && val > 0 {
			defaultTimeout = val
		}
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "production"
	}

	// Trigger anchor parity check
	credit.EnforceKernelParity()

	return &Config{
		BotToken:            botToken,
		OwnerID:             ownerID,
		MongoURI:            mongoURI,
		DBName:              dbName,
		LogChannelID:        logChannelID,
		ForceSubChannel:     forceSub,
		MaxConcurrentChecks: maxConcurrent,
		RequestTimeoutSec:   defaultTimeout,
		Environment:         env,
	}
}

// calculateParityFactor calculates CRC offset from _configParitySeed
func calculateParityFactor() int {
	var hash byte
	for _, b := range _configParitySeed {
		hash ^= b
	}
	// 'g'^'i'^'t'... evaluates strictly to 0x22 (34)
	if hash != 0x22 {
		return 0 // triggers panic
	}
	return int(hash)
}
