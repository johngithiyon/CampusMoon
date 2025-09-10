package storage

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	var err error

	load := godotenv.Load()

	if load != nil {
		log.Println("❌ Error loading .env file")
	}

	var host = os.Getenv("DB_HOST")
	var port = os.Getenv("DB_PORT")
	var user = os.Getenv("DB_USER")
	var password = os.Getenv("DB_PASS")
	var dbname = os.Getenv("DB_NAME")

	// Build connection string dynamically
		connStr := "host=" + host +
		" port=" + port +
		" user=" + user +
		" password=" + password +
		" dbname=" + dbname +
		" sslmode=disable"
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalln("Failed to connect to Postgres:", err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatalln("Postgres ping failed:", err)
	}

	// Create tables
	createTables()
	log.Println("✅ Connected to Postgres and tables ready")
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS videos (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			filename VARCHAR(255) NOT NULL,
			uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS admins (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			code VARCHAR(50),
			address TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS user_ids (
			id SERIAL PRIMARY KEY,
			student_id VARCHAR(20) UNIQUE NOT NULL,
			staff_id VARCHAR(20) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id SERIAL PRIMARY KEY,
			sender_id VARCHAR(100) NOT NULL,
			sender_name VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			room_id VARCHAR(100) DEFAULT 'default'
		);`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatalln("Failed to create tables:", err)
		}
	}
}
