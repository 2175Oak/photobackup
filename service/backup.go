package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"photo-backup/fileops"
	"photo-backup/gemini"
	"photo-backup/model"
	"photo-backup/repository"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

var (
	errNoSource = errors.New("ยังไม่ได้เลือกโฟลเดอร์ต้นทาง")
	errNoDest   = errors.New("ยังไม่ได้เลือกโฟลเดอร์ปลายทาง")
	errBusy     = errors.New("กำลังทำงานอื่นอยู่ กรุณารอให้เสร็จก่อน")
)

// PhotoService คือส่วนที่หน้าจอ (Nuxt) เรียกใช้ผ่าน Wails
// ทุก method ที่ขึ้นต้นด้วยตัวพิมพ์ใหญ่จะถูกเปิดให้ฝั่ง JavaScript เรียกได้
type PhotoService struct {
	ctx      context.Context
	photos   *repository.PhotoRepository
	settings *repository.SettingRepository

	mu     sync.RWMutex // ป้องกัน source/dest ถูกอ่าน-เขียนพร้อมกัน
	source string
	dest   string

	opMu sync.Mutex // ให้ย้าย/ลบ/ล้างข้อมูล ทำได้ทีละงาน

	thumbSem chan struct{} // จำกัดจำนวนรูปที่ถูกย่อพร้อมกัน ไม่ให้กินหน่วยความจำมาก

	gemini        *gemini.Client     // วิเคราะห์ภาพ (อ่าน GEMINI_API_KEY จาก .env)
	cancelAnalyze context.CancelFunc // หยุดงานวิเคราะห์ที่กำลังทำอยู่ (ป้องกันด้วย mu)
}

func NewPhotoService(db *gorm.DB) *PhotoService {
	s := &PhotoService{
		photos:   repository.NewPhotoRepository(db),
		settings: repository.NewSettingRepository(db),
		thumbSem: make(chan struct{}, 3),
		gemini:   gemini.New(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL")),
	}
	// จำโฟลเดอร์ที่ใช้ครั้งล่าสุด
	if cfg, err := s.settings.Load(); err == nil {
		s.source, s.dest = cfg.Source, cfg.Dest
	}
	return s
}

func (s *PhotoService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *PhotoService) snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return State{Source: s.source, Dest: s.dest}
}

func (s *PhotoService) emit(event string, data any) {
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, event, data)
	}
}

func toFileInfo(f fileops.Info, inDest bool) FileInfo {
	return FileInfo{
		Name:    f.Name,
		Size:    f.Size,
		ModTime: f.ModTime.Format(time.RFC3339),
		InDest:  inDest,
	}
}

// GetState คืนโฟลเดอร์ต้นทาง/ปลายทางที่จำไว้ (เรียกตอนเปิดหน้าจอ)
func (s *PhotoService) GetState() State {
	return s.snapshot()
}

// SelectFolder เปิดกล่องเลือกโฟลเดอร์ คืนค่า path ที่เลือก (ว่างถ้ากดยกเลิก)
func (s *PhotoService) SelectFolder(title string) (string, error) {
	if title == "" {
		title = "เลือกโฟลเดอร์"
	}
	return runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{Title: title})
}

// SetSource ตั้งโฟลเดอร์ต้นทาง (เทียบเท่าคำสั่ง /source)
func (s *PhotoService) SetSource(path string) (SourceInfo, error) {
	p, err := fileops.NormalizePath(path)
	if err != nil {
		return SourceInfo{}, err
	}
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return SourceInfo{}, fmt.Errorf("ไม่พบโฟลเดอร์นี้: %s", p)
	}
	imgs, err := fileops.ListImages(p)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("อ่านโฟลเดอร์ต้นทางไม่ได้: %w", err)
	}
	if fileops.SameDir(p, s.snapshot().Dest) {
		return SourceInfo{}, errors.New("โฟลเดอร์ต้นทางต้องไม่ใช่โฟลเดอร์เดียวกับปลายทาง")
	}

	s.mu.Lock()
	s.source = p
	s.mu.Unlock()

	info := SourceInfo{Path: p, Count: len(imgs)}
	if err := s.settings.SaveSource(p); err != nil {
		info.Warning = "ใช้โฟลเดอร์นี้ได้ แต่บันทึกการตั้งค่าลงฐานข้อมูลไม่สำเร็จ: " + err.Error()
	}
	return info, nil
}

// SetDest ตั้งโฟลเดอร์ปลายทาง สร้างให้ถ้ายังไม่มี และตรวจความถูกต้องของรูปที่เคยสำรองไว้ทันที
// (เทียบเท่าคำสั่ง /dest)
func (s *PhotoService) SetDest(path string) (DestInfo, error) {
	p, err := fileops.NormalizePath(path)
	if err != nil {
		return DestInfo{}, err
	}
	if fileops.SameDir(p, s.snapshot().Source) {
		return DestInfo{}, errors.New("โฟลเดอร์ปลายทางต้องไม่ใช่โฟลเดอร์เดียวกับต้นทาง")
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		return DestInfo{}, fmt.Errorf("สร้างหรือเข้าถึงโฟลเดอร์ปลายทางไม่ได้: %w", err)
	}

	s.mu.Lock()
	s.dest = p
	s.mu.Unlock()

	info := DestInfo{Path: p, Integrity: []IntegrityItem{}}
	if err := s.settings.SaveDest(p); err != nil {
		info.Warning = "ใช้โฟลเดอร์นี้ได้ แต่บันทึกการตั้งค่าลงฐานข้อมูลไม่สำเร็จ: " + err.Error()
	}
	items, err := s.checkIntegrity(p)
	if err != nil {
		info.Warning = strings.TrimSpace(info.Warning + " ตรวจความถูกต้องไม่สำเร็จ: " + err.Error())
	} else {
		info.Integrity = items
	}
	return info, nil
}

// ListSource แสดงรูปในโฟลเดอร์ต้นทาง (เทียบเท่า /list source)
// InDest = true หมายถึงรูปชื่อนี้มีที่ปลายทางหรือในฐานข้อมูลแล้ว (ถ้าย้ายจะถูกข้าม)
func (s *PhotoService) ListSource() ([]FileInfo, error) {
	st := s.snapshot()
	if st.Source == "" {
		return nil, errNoSource
	}
	imgs, err := fileops.ListImages(st.Source)
	if err != nil {
		return nil, fmt.Errorf("อ่านโฟลเดอร์ต้นทางไม่ได้: %w", err)
	}

	var logged map[string]bool
	if st.Dest != "" {
		if logged, err = s.photos.NamesByDest(st.Dest); err != nil {
			return nil, err
		}
	}

	out := make([]FileInfo, 0, len(imgs))
	for _, f := range imgs {
		inDest := logged[f.Name]
		if !inDest && st.Dest != "" {
			inDest = fileops.Exists(filepath.Join(st.Dest, f.Name))
		}
		out = append(out, toFileInfo(f, inDest))
	}
	return out, nil
}

// ListDest แสดงรูปในโฟลเดอร์ปลายทาง (เทียบเท่า /list dest)
func (s *PhotoService) ListDest() ([]FileInfo, error) {
	st := s.snapshot()
	if st.Dest == "" {
		return nil, errNoDest
	}
	imgs, err := fileops.ListImages(st.Dest)
	if err != nil {
		return nil, fmt.Errorf("อ่านโฟลเดอร์ปลายทางไม่ได้: %w", err)
	}
	out := make([]FileInfo, 0, len(imgs))
	for _, f := range imgs {
		out = append(out, toFileInfo(f, true))
	}
	return out, nil
}

// ListHistory แสดงประวัติการสำรองของปลายทางหนึ่งๆ (เทียบเท่า /list db [path])
// ถ้าไม่ระบุ dest จะใช้ปลายทางปัจจุบัน ประวัติของปลายทางอื่นจะไม่ปนมา
func (s *PhotoService) ListHistory(dest string) ([]model.Photo, error) {
	target := s.snapshot().Dest
	if strings.TrimSpace(dest) != "" {
		p, err := fileops.NormalizePath(dest)
		if err != nil {
			return nil, err
		}
		target = p
	}
	if target == "" {
		return nil, errNoDest
	}
	return s.photos.ListByDest(target)
}

// GetThumbnail คืนรูปตัวอย่าง (JPEG ย่อแล้ว) ในรูปแบบ data URL ให้หน้าจอแสดงใน <img> ได้เลย
// kind คือ "source" หรือ "dest" ส่วน name คือชื่อไฟล์ในโฟลเดอร์นั้น
// รูปที่ถอดรหัสไม่ได้ (เช่น HEIC) จะคืน error ให้หน้าจอแสดงช่องว่างแทน
func (s *PhotoService) GetThumbnail(kind, name string) (string, error) {
	st := s.snapshot()
	var dir string
	switch kind {
	case "source":
		dir = st.Source
	case "dest":
		dir = st.Dest
	}

	// name ต้องเป็นชื่อไฟล์ล้วนๆ เพื่อกันการเข้าถึงไฟล์นอกโฟลเดอร์ (เช่น ../)
	if dir == "" || name == "" || name != filepath.Base(name) || !fileops.IsImage(name) {
		return "", errors.New("ไม่พบรูปนี้")
	}
	full := filepath.Join(dir, name)
	if !fileops.Exists(full) {
		return "", errors.New("ไม่พบไฟล์รูปนี้")
	}

	s.thumbSem <- struct{}{}
	defer func() { <-s.thumbSem }()

	data, err := fileops.Thumbnail(full, 360)
	if err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
}
