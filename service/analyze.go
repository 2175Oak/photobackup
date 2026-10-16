package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"photo-backup/fileops"
	"photo-backup/gemini"
	"photo-backup/model"
)

const (
	analyzeMaxSide  = 1280             // ย่อรูปก่อนส่ง ประหยัดเน็ตและโควตา คำอธิบายก็ยังแม่นพอ
	analyzeMaxRaw   = 15 * 1024 * 1024 // รูปที่ย่อเองไม่ได้ (HEIC/WebP) ส่งไฟล์ดิบได้ไม่เกินนี้
	analyzeTimeout  = 90 * time.Second
	analyzeProgress = "analyze:progress"
)

// AnalysisStatus บอกว่าตอนนี้วิเคราะห์ภาพได้หรือยัง และวิเคราะห์ไปแล้วกี่รูป
type AnalysisStatus struct {
	Configured bool   `json:"configured"` // มี GEMINI_API_KEY หรือไม่
	Model      string `json:"model"`
	Total      int64  `json:"total"`    // รูปในประวัติของปลายทางปัจจุบัน
	Analyzed   int64  `json:"analyzed"` // รูปที่มีคำอธิบายแล้ว
}

type AnalyzeResult struct {
	Done      int          `json:"done"`
	Failed    int          `json:"failed"`
	Cancelled bool         `json:"cancelled"`
	Items     []FileResult `json:"items"`
}

// GetAnalysisStatus ใช้แสดงสถานะบนหน้าค้นหา
func (s *PhotoService) GetAnalysisStatus() (AnalysisStatus, error) {
	st := AnalysisStatus{Configured: s.gemini.Configured(), Model: s.gemini.Model()}
	dest := s.snapshot().Dest
	if dest == "" {
		return st, nil
	}
	var err error
	st.Total, st.Analyzed, err = s.photos.CountAnalysis(dest)
	return st, err
}

// AnalyzePending ให้ Gemini อธิบายและตั้งแท็กให้รูปของปลายทางปัจจุบันที่ยังไม่เคยวิเคราะห์
// ทำทีละรูปและส่งความคืบหน้าผ่านอีเวนต์ "analyze:progress" รูปที่พลาดจะยังไม่มีคำอธิบาย
// จึงกดวิเคราะห์ซ้ำได้โดยไม่ต้องทำรูปที่สำเร็จแล้วใหม่
func (s *PhotoService) AnalyzePending() (AnalyzeResult, error) {
	if !s.gemini.Configured() {
		return AnalyzeResult{}, gemini.ErrNoKey
	}
	if !s.opMu.TryLock() {
		return AnalyzeResult{}, errBusy
	}
	defer s.opMu.Unlock()

	dest := s.snapshot().Dest
	if dest == "" {
		return AnalyzeResult{}, errNoDest
	}
	pending, err := s.photos.PendingAnalysis(dest)
	if err != nil {
		return AnalyzeResult{}, err
	}
	if len(pending) == 0 {
		return AnalyzeResult{}, errors.New("ไม่มีรูปที่ต้องวิเคราะห์ (วิเคราะห์ครบแล้ว)")
	}

	base := s.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	s.mu.Lock()
	s.cancelAnalyze = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.cancelAnalyze = nil
		s.mu.Unlock()
	}()

	res := AnalyzeResult{Items: make([]FileResult, 0, len(pending))}
	for i, p := range pending {
		if ctx.Err() != nil {
			res.Cancelled = true
			break
		}
		item := FileResult{Name: p.FileName}
		fatal := false

		if err := s.analyzeOne(ctx, dest, p); err != nil {
			item.Status = StatusFailed
			item.Message = err.Error()
			var ae *gemini.APIError
			fatal = errors.As(err, &ae) && ae.Fatal()
			if ctx.Err() != nil {
				res.Cancelled = true
				item.Message = "ยกเลิกการวิเคราะห์"
			}
			res.Failed++
		} else {
			item.Status = StatusMoved // ใช้สถานะ "สำเร็จ" ร่วมกับส่วนอื่น
			res.Done++
		}
		res.Items = append(res.Items, item)
		s.emit(analyzeProgress, ProgressEvent{Done: i + 1, Total: len(pending), Name: p.FileName, Status: item.Status})

		if fatal { // key ผิด/ไม่มีสิทธิ์: ไม่มีประโยชน์ที่จะยิงรูปที่เหลือ
			break
		}
		if res.Cancelled {
			break
		}
	}
	return res, nil
}

// CancelAnalyze หยุดงานวิเคราะห์ที่กำลังทำอยู่ (รูปที่เสร็จแล้วยังอยู่ครบ)
func (s *PhotoService) CancelAnalyze() {
	s.mu.RLock()
	cancel := s.cancelAnalyze
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (s *PhotoService) analyzeOne(ctx context.Context, dest string, p model.Photo) error {
	if p.FileName != filepath.Base(p.FileName) || !fileops.IsImage(p.FileName) {
		return errors.New("ชื่อไฟล์ไม่ถูกต้อง")
	}
	full := filepath.Join(dest, p.FileName)
	if !fileops.Exists(full) {
		return errors.New("ไม่พบไฟล์รูปในปลายทาง")
	}

	data, mime, err := analysisImage(full)
	if err != nil {
		return err
	}

	cctx, cancel := context.WithTimeout(ctx, analyzeTimeout)
	defer cancel()
	out, err := s.gemini.Describe(cctx, data, mime)
	if err != nil {
		return err
	}
	if err := s.photos.SaveAnalysis(p.ID, out.Caption, out.Tags); err != nil {
		return fmt.Errorf("วิเคราะห์แล้วแต่บันทึกลงฐานข้อมูลไม่สำเร็จ: %w", err)
	}
	return nil
}

// analysisImage เตรียมภาพที่จะส่งให้ AI: JPEG/PNG/GIF ย่อด้วย Go (และหมุนตาม EXIF)
// ส่วน WebP/HEIC ที่ Go อ่านไม่ได้ ส่งไฟล์ดิบไปให้ Gemini อ่านเอง
func analysisImage(path string) ([]byte, string, error) {
	if data, err := fileops.Thumbnail(path, analyzeMaxSide); err == nil {
		return data, "image/jpeg", nil
	}
	mime := map[string]string{".webp": "image/webp", ".heic": "image/heic", ".heif": "image/heif"}[strings.ToLower(filepath.Ext(path))]
	if mime == "" {
		return nil, "", errors.New("ไฟล์ชนิดนี้ส่งให้ AI วิเคราะห์ไม่ได้")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if fi.Size() > analyzeMaxRaw {
		return nil, "", errors.New("ไฟล์ใหญ่เกินไปสำหรับส่งให้ AI วิเคราะห์")
	}
	data, err := os.ReadFile(path)
	return data, mime, err
}

// ---- ค้นหา ----

// SearchPhotos ค้นรูปของปลายทางปัจจุบันจากคำอธิบาย และ/หรือแท็ก
//   - text: คำค้นหาอิสระ แยกด้วยช่องว่างได้หลายคำ (ทุกคำต้องอยู่ในคำอธิบาย)
//   - tags: แท็กที่เลือกจาก Tag Cloud (รูปต้องมีครบทุกแท็ก)
func (s *PhotoService) SearchPhotos(text string, tags []string) ([]model.Photo, error) {
	dest := s.snapshot().Dest
	if dest == "" {
		return nil, errNoDest
	}
	return s.photos.Search(dest, strings.Fields(text), gemini.CleanTags(tags))
}

// ListTags คืนแท็กทั้งหมดพร้อมจำนวนรูป สำหรับวาด Tag Cloud
func (s *PhotoService) ListTags() ([]model.TagCount, error) {
	dest := s.snapshot().Dest
	if dest == "" {
		return nil, errNoDest
	}
	return s.photos.TagCounts(dest)
}
