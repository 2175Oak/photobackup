package model

import "time"

// Photo คือรูปหนึ่งรูปที่ถูกย้ายมาเก็บไว้ในโฟลเดอร์ปลายทาง (Dest)
// Dest ใช้แยกประวัติของแต่ละปลายทางออกจากกัน
type Photo struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	FileName   string    `gorm:"not null" json:"fileName"`
	Dest       string    `gorm:"index" json:"dest"`
	BackupPath string    `gorm:"not null;uniqueIndex" json:"backupPath"`
	Caption    string    `json:"caption"`
	BackedUpAt time.Time `gorm:"autoCreateTime" json:"backedUpAt"`
	Tags       []Tag     `gorm:"many2many:photo_tag;" json:"tags"`
}

type Tag struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex;not null" json:"name"`
}

// Setting เก็บโฟลเดอร์ต้นทาง/ปลายทางล่าสุด ใช้แถวเดียว (ID = 1)
// เพื่อให้เปิดโปรแกรมครั้งต่อไปแล้วจำค่าเดิมได้
type Setting struct {
	ID     uint `gorm:"primaryKey"`
	Source string
	Dest   string
}
