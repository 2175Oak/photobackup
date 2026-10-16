package model

import "time"

type Photo struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	FileName   string    `gorm:"not null" json:"fileName"`
	BackupPath string    `gorm:"not null;uniqueIndex" json:"backupPath"`
	Caption    string    `json:"caption"`
	BackedUpAt time.Time `gorm:"autoCreateTime" json:"backedUpAt"`
	Tags       []Tag     `gorm:"many2many:photo_tag;" json:"tags"`
}

type Tag struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex;not null" json:"name"`
}
