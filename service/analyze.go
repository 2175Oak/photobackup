package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"photo-backup/fileops"
	"photo-backup/gemini"
	"photo-backup/model"
)

const (
	analyzeMaxSide  = 768              // ย่อภาพก่อนส่ง ลดข้อมูลสำหรับงานคำอธิบายและแท็ก
	analyzeMaxRaw   = 15 * 1024 * 1024 // รูปที่ย่อเองไม่ได้ (HEIC/WebP) ส่งไฟล์ดิบได้ไม่เกินนี้
	analyzeProgress = "analyze:progress"
)

// AnalysisStatus บอกว่าตอนนี้วิเคราะห์ภาพได้หรือยัง และวิเคราะห์ไปแล้วกี่รูป
type AnalysisStatus struct {
	BatchSize  int    `json:"batchSize"`
	Workers    int    `json:"workers"`
	RPM        int    `json:"rpm"`
	TPM        int    `json:"tpm"`
	Configured bool   `json:"configured"` // มี GEMINI_API_KEY หรือไม่
	Model      string `json:"model"`
	Total      int64  `json:"total"`    // รูปในประวัติของปลายทางปัจจุบัน
	Analyzed   int64  `json:"analyzed"` // รูปที่มีคำอธิบายแล้ว
}

type AnalyzeResult struct {
	Metrics   AnalysisMetrics `json:"metrics"`
	Done      int             `json:"done"`
	Failed    int             `json:"failed"`
	Cancelled bool            `json:"cancelled"`
	Items     []FileResult    `json:"items"`
}

// Analysis progress includes actual outcomes, rather than only attempted files.
type analysisProgressEvent struct {
	ProgressEvent
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	Message   string `json:"message"`
	Stopped   bool   `json:"stopped"`
}

// GetAnalysisStatus ใช้แสดงสถานะบนหน้าค้นหา
func (s *PhotoService) GetAnalysisStatus() (AnalysisStatus, error) {
	st := AnalysisStatus{Configured: s.batch.Configured(), Model: s.batch.Model()}
	st.BatchSize, st.Workers, st.RPM, st.TPM = s.batch.Settings().Size, s.batch.Settings().Workers, s.batch.Settings().RPM, s.batch.Settings().TPM
	dest := s.snapshot().Dest
	if dest == "" {
		return st, nil
	}
	var err error
	st.Total, st.Analyzed, err = s.photos.CountAnalysis(dest)
	return st, err
}

// AnalyzePending ให้ Gemini อธิบายและตั้งแท็กให้รูปของปลายทางปัจจุบันที่ยังไม่เคยวิเคราะห์
// ใช้คิวเป็นชุดและส่งความคืบหน้าผ่านอีเวนต์ "analyze:progress"
// จึงกดวิเคราะห์ซ้ำได้โดยไม่ต้องทำรูปที่สำเร็จแล้วใหม่
func (s *PhotoService) AnalyzePending() (AnalyzeResult, error) {
	return s.analyzePending(0)
}

// AnalyzeSample saves real results for at most 100 pending photos, so the
// following full run skips them instead of paying for the same work twice.
func (s *PhotoService) AnalyzeSample() (AnalyzeResult, error) { return s.analyzePending(100) }

func (s *PhotoService) analyzePending(limit int) (AnalyzeResult, error) {
	if !s.batch.Configured() {
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
	if limit > 0 && len(pending) > limit {
		pending = pending[:limit]
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

	s.emit(analyzeProgress, analysisProgressEvent{ProgressEvent: ProgressEvent{Done: 0, Total: len(pending)}})
	if err := s.photos.RecoverAnalysis(dest); err != nil {
		return AnalyzeResult{}, err
	}
	return s.analyzeTagQueue(ctx, dest, pending), nil
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
//   - text: คำค้นหาคั่นด้วยช่องว่าง แต่ละคำต้องตรงหรือสะกดใกล้เคียงกับ caption/tag
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
