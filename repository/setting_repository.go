package repository

import (
	"photo-backup/model"

	"gorm.io/gorm"
)

// SettingRepository จำโฟลเดอร์ต้นทาง/ปลายทางล่าสุดไว้ในแถวเดียว (ID = 1)
// เรียกกี่ครั้งก็ไม่สร้างแถวซ้ำ และการตั้งฝั่งหนึ่งจะไม่เขียนทับอีกฝั่ง
type SettingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// Load คืนค่าที่บันทึกไว้ ถ้ายังไม่เคยบันทึกจะคืนค่าว่าง
func (r *SettingRepository) Load() (model.Setting, error) {
	var rows []model.Setting
	// ใช้ Find + Limit แทน First เพื่อไม่ให้มี log "record not found" เวลายังไม่มีข้อมูล
	if err := r.db.Where("id = ?", 1).Limit(1).Find(&rows).Error; err != nil {
		return model.Setting{}, err
	}
	if len(rows) == 0 {
		return model.Setting{ID: 1}, nil
	}
	return rows[0], nil
}

func (r *SettingRepository) SaveSource(path string) error {
	cfg, err := r.Load()
	if err != nil {
		return err
	}
	cfg.Source = path
	return r.db.Save(&cfg).Error
}

func (r *SettingRepository) SaveDest(path string) error {
	cfg, err := r.Load()
	if err != nil {
		return err
	}
	cfg.Dest = path
	return r.db.Save(&cfg).Error
}

// DeleteAll ลบการตั้งค่าที่จำไว้ทั้งหมด คืนจำนวนแถวที่ลบ
func (r *SettingRepository) DeleteAll() (int64, error) {
	res := r.db.Where("1 = 1").Delete(&model.Setting{})
	return res.RowsAffected, res.Error
}
