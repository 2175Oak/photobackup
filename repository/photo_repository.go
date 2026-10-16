package repository

import (
	"strings"

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

// ---- วิเคราะห์ภาพ / ค้นหา ----

// PendingAnalysis คืนรูปของปลายทางนี้ที่ยังไม่มีคำอธิบาย (ยังไม่เคยวิเคราะห์) เรียงจากเก่าไปใหม่
func (r *PhotoRepository) PendingAnalysis(dest string) ([]model.Photo, error) {
	photos := []model.Photo{}
	err := r.db.Where("dest = ? AND (caption IS NULL OR caption = '')", dest).Order("id ASC").Find(&photos).Error
	return photos, err
}

// CountAnalysis คืนจำนวนรูปทั้งหมดและจำนวนที่วิเคราะห์แล้วของปลายทางนี้
func (r *PhotoRepository) CountAnalysis(dest string) (total, analyzed int64, err error) {
	if err = r.db.Model(&model.Photo{}).Where("dest = ?", dest).Count(&total).Error; err != nil {
		return
	}
	err = r.db.Model(&model.Photo{}).Where("dest = ? AND caption <> ''", dest).Count(&analyzed).Error
	return
}

// SaveAnalysis บันทึกคำอธิบายและแท็กของรูป (แทนที่แท็กเดิมทั้งหมด) ใน transaction เดียว
// แท็กชื่อซ้ำจะใช้แถวเดิมในตาราง tags ไม่สร้างใหม่
func (r *PhotoRepository) SaveAnalysis(photoID int64, caption string, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Photo{}).Where("id = ?", photoID).Update("caption", caption).Error; err != nil {
			return err
		}
		tags := make([]model.Tag, 0, len(tagNames))
		for _, name := range tagNames {
			t := model.Tag{}
			if err := tx.Where("name = ?", name).FirstOrCreate(&t, model.Tag{Name: name}).Error; err != nil {
				return err
			}
			tags = append(tags, t)
		}
		return tx.Model(&model.Photo{ID: photoID}).Association("Tags").Replace(tags)
	})
}

// escapeLike ทำให้ % _ \ ในคำค้นหาเป็นตัวอักษรธรรมดา ไม่ใช่ wildcard ของ LIKE
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Search ค้นรูปของปลายทางนี้ที่มีคำอธิบายเท่านั้น
//   - words: ทุกคำต้องอยู่ในคำอธิบาย (ไม่สนตัวพิมพ์เล็ก/ใหญ่ของภาษาอังกฤษ)
//   - tags : รูปต้องมีครบทุกแท็กที่เลือก
//
// ถ้าไม่ส่งทั้งสองอย่างจะคืนรูปที่วิเคราะห์แล้วทั้งหมด
func (r *PhotoRepository) Search(dest string, words, tags []string) ([]model.Photo, error) {
	q := r.db.Preload("Tags").Where("dest = ? AND caption <> ''", dest)
	for _, w := range words {
		q = q.Where(`caption LIKE ? ESCAPE '\'`, "%"+escapeLike(w)+"%")
	}
	if len(tags) > 0 {
		q = q.Where(
			`id IN (SELECT pt.photo_id FROM photo_tag pt JOIN tags t ON t.id = pt.tag_id
			        WHERE t.name IN ? GROUP BY pt.photo_id HAVING COUNT(DISTINCT t.id) = ?)`,
			tags, len(tags),
		)
	}
	photos := []model.Photo{}
	err := q.Order("id DESC").Find(&photos).Error
	return photos, err
}

// TagCounts คืนแท็กทั้งหมดของรูปในปลายทางนี้พร้อมจำนวนรูป เรียงจากมากไปน้อย
func (r *PhotoRepository) TagCounts(dest string) ([]model.TagCount, error) {
	out := []model.TagCount{}
	err := r.db.Raw(`
		SELECT t.name AS name, COUNT(DISTINCT p.id) AS count
		FROM tags t
		JOIN photo_tag pt ON pt.tag_id = t.id
		JOIN photos p ON p.id = pt.photo_id
		WHERE p.dest = ?
		GROUP BY t.id, t.name
		ORDER BY count DESC, t.name ASC`, dest).Scan(&out).Error
	return out, err
}
