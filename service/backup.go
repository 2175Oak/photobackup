package service

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"photo-backup/model"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type PhotoService struct {
	ctx context.Context
	db  *gorm.DB
}

func NewPhotoService(db *gorm.DB) *PhotoService {
	return &PhotoService{db: db}
}

func (s *PhotoService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// เปิดกล่องเลือกโฟลเดอร์ คืนค่า path ที่เลือก
func (s *PhotoService) SelectFolder() (string, error) {
	return runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "เลือกโฟลเดอร์ต้นทาง",
	})
}

// คัดลอกรูปจาก sourceDir ไปที่โฟลเดอร์ backup แล้วบันทึกลง DB คืนจำนวนรูปที่ Backup ใหม่
func (s *PhotoService) Backup(sourceDir string) (int, error) {
	if err := os.MkdirAll("backup", 0755); err != nil {
		return 0, err
	}

	count := 0
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return nil
		}

		dest := filepath.Join("backup", d.Name())
		if _, err := os.Stat(dest); err == nil {
			return nil // ชื่อซ้ำ ข้ามไป
		}

		if err := copyFile(path, dest); err != nil {
			return err
		}
		photo := model.Photo{FileName: d.Name(), BackupPath: dest}
		if err := s.db.Create(&photo).Error; err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
