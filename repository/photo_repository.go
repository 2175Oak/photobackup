package repository

import (
	"photo-backup/model"

	"gorm.io/gorm"
)

// PhotoRepository รวมคำสั่งฐานข้อมูลของตาราง photos
// ทุกคำสั่งกรองด้วย dest เพื่อให้ประวัติของแต่ละปลายทางไม่ปนกัน
type PhotoRepository struct {
	db *gorm.DB
}

func NewPhotoRepository(db *gorm.DB) *PhotoRepository {
	return &PhotoRepository{db: db}
}

// NamesByDest คืนชุดชื่อไฟล์ที่บันทึกไว้ของปลายทางนี้ (query ครั้งเดียว ใช้ตรวจไฟล์ซ้ำทีละไฟล์ได้เร็ว)
func (r *PhotoRepository) NamesByDest(dest string) (map[string]bool, error) {
	var names []string
	if err := r.db.Model(&model.Photo{}).Where("dest = ?", dest).Pluck("file_name", &names).Error; err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

// ListByDest คืนประวัติของปลายทางนี้ เรียงจากใหม่ไปเก่า
func (r *PhotoRepository) ListByDest(dest string) ([]model.Photo, error) {
	photos := []model.Photo{}
	err := r.db.Preload("Tags").Where("dest = ?", dest).Order("id DESC").Find(&photos).Error
	return photos, err
}

// CreateBatch บันทึกหลายรายการในคราวเดียว (เร็วกว่าบันทึกทีละแถวมาก)
func (r *PhotoRepository) CreateBatch(photos []model.Photo) error {
	return r.db.Omit("Tags").CreateInBatches(&photos, 100).Error
}

func (r *PhotoRepository) Create(photo *model.Photo) error {
	return r.db.Omit("Tags").Create(photo).Error
}

// DeleteByDestAndName ลบประวัติของรูปหนึ่งรูป (รวมความสัมพันธ์กับ Tag) คืนจำนวนแถวที่ลบ
func (r *PhotoRepository) DeleteByDestAndName(dest, name string) (int64, error) {
	var affected int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM photo_tag WHERE photo_id IN (SELECT id FROM photos WHERE dest = ? AND file_name = ?)",
			dest, name,
		).Error; err != nil {
			return err
		}
		res := tx.Where("dest = ? AND file_name = ?", dest, name).Delete(&model.Photo{})
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

// DeleteByDest ล้างประวัติทั้งหมดของปลายทางนี้ (ไม่แตะไฟล์จริงบนดิสก์)
func (r *PhotoRepository) DeleteByDest(dest string) (int64, error) {
	var affected int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM photo_tag WHERE photo_id IN (SELECT id FROM photos WHERE dest = ?)",
			dest,
		).Error; err != nil {
			return err
		}
		res := tx.Where("dest = ?", dest).Delete(&model.Photo{})
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}

// DeleteAll ล้างตาราง photos, tags และ photo_tag ทั้งหมด (ไม่แตะไฟล์จริงบนดิสก์)
func (r *PhotoRepository) DeleteAll() (photos int64, tags int64, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM photo_tag").Error; err != nil {
			return err
		}
		res := tx.Where("1 = 1").Delete(&model.Photo{})
		if res.Error != nil {
			return res.Error
		}
		photos = res.RowsAffected

		res = tx.Where("1 = 1").Delete(&model.Tag{})
		if res.Error != nil {
			return res.Error
		}
		tags = res.RowsAffected
		return nil
	})
	return photos, tags, err
}
