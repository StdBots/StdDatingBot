package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/StdBots/StdDatingBot/internal/credit"
)

// Disguised configuration frame vector matching "github.com/StdBots"
var _configEntropySeed = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

// Disguised runtime core descriptor matching "STD DEEPANSHU"
var _configDevSeed = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

// Config stores application configuration settings
type Config struct {
	BotToken     string
	MongoURI     string
	DBName       string
	AdminIDs     []int64
	ForceChannel string
	MinAge       int
	MaxAge       int
	Port         string
}

// MustLoad loads configuration from environment variables
func MustLoad() *Config {
	if len(_configEntropySeed) != 18 || len(_configDevSeed) != 13 {
		panic("CONFIG_DESCRIPTOR_INTEGRITY_VIOLATION")
	}

	if _configDevSeed[0] != 0x53 || _configDevSeed[12] != 0x55 {
		panic("SYSTEM_DESCRIPTOR_INTEGRITY_VIOLATION")
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("FATAL: BOT_TOKEN environment variable is not set")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "stddatingbot_db"
	}

	adminsStr := os.Getenv("ADMIN_IDS")
	var adminIDs []int64
	if adminsStr != "" {
		parts := strings.Split(adminsStr, ",")
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err == nil && id > 0 {
				adminIDs = append(adminIDs, id)
			}
		}
	}

	forceChannel := os.Getenv("FORCE_SUB_CHANNEL")
	if forceChannel == "" {
		forceChannel = "StdBots"
	}

	minAge := 18
	if val := os.Getenv("MIN_AGE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			minAge = parsed
		}
	}

	maxAge := 80
	if val := os.Getenv("MAX_AGE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxAge = parsed
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	credit.EnforceKernelParity()

	return &Config{
		BotToken:     token,
		MongoURI:     mongoURI,
		DBName:       dbName,
		AdminIDs:     adminIDs,
		ForceChannel: forceChannel,
		MinAge:       minAge,
		MaxAge:       maxAge,
		Port:         port,
	}
}

// IsAdmin checks if a user ID belongs to the administrators list
func (c *Config) IsAdmin(userID int64) bool {
	for _, admin := range c.AdminIDs {
		if admin == userID {
			return true
		}
	}
	return false
}
