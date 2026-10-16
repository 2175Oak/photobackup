package repository

import (
	"log"
	"os"
	"path/filepath"
	"photo-backup/model"

	"github.com/glebarez/sqlite"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var db *gorm.DB

func NewDbConnection() (*gorm.DB, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file:", err)
		return nil, err
	}

	dbPath := os.Getenv("DATABASE")
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dbPath = filepath.Join(dir, dbPath)
	log.Println("Database path:", dbPath)

	// สร้างโฟลเดอร์ database ถ้ายังไม่มี
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Println("Error connecting to database:", err)
		return nil, err
	}

	// สร้าง/อัปเดตตารางอัตโนมัติจาก Model
	if err := database.AutoMigrate(&model.Photo{}, &model.Tag{}); err != nil {
		log.Println("Error migrating:", err)
		return nil, err
	}
	db = database
	return db, nil
}
