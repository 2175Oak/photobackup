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

func NewDbConnection() (*gorm.DB, error) {
	// ไม่มี .env ก็ไม่เป็นไร จะใช้ค่าเริ่มต้นด้านล่าง
	// (แอปที่ build แล้วอาจถูกเปิดจากโฟลเดอร์อื่นที่ไม่มีไฟล์นี้)
	if err := godotenv.Load(); err != nil {
		log.Println("ไม่พบไฟล์ .env ใช้ค่าเริ่มต้นแทน:", err)
	}

	dbPath := os.Getenv("DATABASE")
	if dbPath == "" {
		dbPath = filepath.Join("database", "photo.db")
	}
	if !filepath.IsAbs(dbPath) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(dir, dbPath)
	}
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
	if err := database.AutoMigrate(&model.Photo{}, &model.Tag{}, &model.Setting{}); err != nil {
		log.Println("Error migrating:", err)
		return nil, err
	}
	return database, nil
}
