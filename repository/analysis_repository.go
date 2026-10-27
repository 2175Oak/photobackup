package repository

import (
	"gorm.io/gorm"
	"photo-backup/model"
)

// Called only while the service owns its operation lock. Interrupted jobs become
// pending again; completed tags and legacy captions are never cleared.
func (r *PhotoRepository) RecoverAnalysis(dest string) error {
	return r.db.Model(&model.Photo{}).Where("dest = ? AND (caption IS NULL OR caption = '') AND COALESCE(analysis_state, '') <> 'done'", dest).
		Updates(map[string]any{"analysis_state": "pending", "analysis_error": ""}).Error
}

func (r *PhotoRepository) AnalysisState(id int64, state, message string, attempt bool) error {
	values := map[string]any{"analysis_state": state, "analysis_error": message}
	if attempt {
		values["analysis_attempts"] = gorm.Expr("COALESCE(analysis_attempts, 0) + 1")
	}
	return r.db.Model(&model.Photo{}).Where("id = ?", id).Updates(values).Error
}

func (r *PhotoRepository) CachedTags(hash, version string) (string, []string, error) {
	var p model.Photo
	err := r.db.Preload("Tags").Where("analysis_hash = ? AND analysis_version = ? AND analysis_state = 'done' AND caption <> ''", hash, version).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	tags := make([]string, 0, len(p.Tags))
	for _, t := range p.Tags {
		tags = append(tags, t.Name)
	}
	return p.Caption, tags, nil
}

func (r *PhotoRepository) SaveTags(id int64, caption string, tags []string, hash, version string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var p model.Photo
		if err := tx.First(&p, id).Error; err != nil {
			return err
		}
		// Keep an existing caption when tags are enriched or restored from cache.
		if p.Caption != "" {
			caption = p.Caption
		}
		if err := NewPhotoRepository(tx).SaveAnalysis(id, caption, tags); err != nil {
			return err
		}
		return tx.Model(&model.Photo{}).Where("id = ?", id).Updates(map[string]any{"analysis_hash": hash, "analysis_version": version}).Error
	})
}

func (r *PhotoRepository) PhotoByID(dest string, id int64) (model.Photo, error) {
	var p model.Photo
	err := r.db.Preload("Tags").Where("dest = ? AND id = ?", dest, id).First(&p).Error
	return p, err
}

func (r *PhotoRepository) SaveCaption(id int64, caption string) error {
	return r.db.Model(&model.Photo{}).Where("id = ?", id).Update("caption", caption).Error
}
