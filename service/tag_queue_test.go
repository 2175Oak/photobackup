package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"photo-backup/gemini"
	"photo-backup/model"
	"sync"
	"testing"
)

type fakeBatch struct {
	mu      sync.Mutex
	calls   [][]int64
	partial bool
	fail    error
	started chan struct{}
}

func pendingPhotos(n int) []model.Photo {
	photos := make([]model.Photo, n)
	for i := range photos {
		photos[i] = model.Photo{ID: int64(i + 1), FileName: fmt.Sprintf("%d.jpg", i)}
	}
	return photos
}

func (f *fakeBatch) Configured() bool { return true }
func (f *fakeBatch) Model() string    { return "fake-tags" }
func (f *fakeBatch) Settings() gemini.BatchConfig {
	return gemini.BatchConfig{Size: 8, Workers: 2, RPM: 14, TPM: 200000, MaxRequests: 200}
}
func (f *fakeBatch) ResetStats() {}
func (f *fakeBatch) Stats() gemini.BatchStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	return gemini.BatchStats{Requests: len(f.calls)}
}
func (f *fakeBatch) Tag(ctx context.Context, images []gemini.TagImage) (map[int64]gemini.Result, error) {
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := []int64{}
	for _, im := range images {
		ids = append(ids, im.ID)
	}
	f.calls = append(f.calls, ids)
	if f.fail != nil {
		return nil, f.fail
	}
	result := map[int64]gemini.Result{}
	for _, im := range images {
		if !(f.partial && len(f.calls) == 1 && im.ID == 2) {
			result[im.ID] = gemini.Result{Caption: "ภาพทดสอบสั้น", Tags: []string{"ทะเล", "ท้องฟ้า", "น้ำ", "กลางแจ้ง", "ธรรมชาติ"}}
		}
	}
	if len(result) != len(images) {
		return result, errors.New("partial")
	}
	return result, nil
}

func tagTestService(t *testing.T, n int, identical bool) (*PhotoService, *fakeBatch) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATABASE", filepath.Join(dir, "test.db"))
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&model.Photo{}, &model.Tag{}, &model.Setting{}); err != nil {
		t.Fatal(err)
	}
	s := NewPhotoService(db)
	s.dest = dir
	f := &fakeBatch{}
	s.batch = f
	photos := pendingPhotos(n)
	for i := range photos {
		p := &photos[i]
		p.Dest = dir
		p.BackupPath = filepath.Join(dir, p.FileName)
		im := image.NewRGBA(image.Rect(0, 0, 8, 8))
		v := uint8(i * 17)
		if identical {
			v = 23
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				im.Set(x, y, color.RGBA{v, 100, 50, 255})
			}
		}
		file, err := os.Create(p.BackupPath)
		if err != nil {
			t.Fatal(err)
		}
		err = jpeg.Encode(file, im, nil)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = s.photos.CreateBatch(photos); err != nil {
		t.Fatal(err)
	}
	return s, f
}

func TestTagQueueRetriesOnlyMissingAndSearchesTagsWithoutCaptions(t *testing.T) {
	s, f := tagTestService(t, 5, false)
	f.partial = true
	// Simulate a previous process dying with a running job.
	if err := s.photos.AnalysisState(2, "running", "old error", true); err != nil {
		t.Fatal(err)
	}
	result, err := s.AnalyzePending()
	if err != nil || result.Done != 5 || result.Failed != 0 {
		t.Fatalf("%+v %v", result, err)
	}
	if len(f.calls) != 2 || len(f.calls[1]) != 1 || f.calls[1][0] != 2 {
		t.Fatalf("repeated successful IDs: %v", f.calls)
	}
	photos, err := s.SearchPhotos("ทะเล", nil)
	if err != nil || len(photos) != 5 {
		t.Fatalf("tags-only search failed: %v %v", photos, err)
	}
	for _, p := range photos {
		if p.Caption != "ภาพทดสอบสั้น" || p.AnalysisState != "done" || len(p.Tags) != 5 {
			t.Fatalf("bad stored result %+v", p)
		}
	}
	st, err := s.GetAnalysisStatus()
	if err != nil || st.Analyzed != 5 {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err = s.AnalyzePending(); err == nil {
		t.Fatal("completed jobs requeued")
	}
}

func TestTagQueueDeduplicatesAndKeepsOriginalCaption(t *testing.T) {
	s, f := tagTestService(t, 12, true)
	if err := s.photos.SaveAnalysis(1, "คำอธิบายเดิม", []string{"เดิม"}); err != nil {
		t.Fatal(err)
	}
	result, err := s.AnalyzePending()
	if err != nil || result.Done != 11 || result.Metrics.Cached != 10 || len(f.calls) != 1 || len(f.calls[0]) != 1 {
		t.Fatalf("result=%+v calls=%v err=%v", result, f.calls, err)
	}
	p, err := s.photos.PhotoByID(s.dest, 1)
	if err != nil || p.Caption != "คำอธิบายเดิม" {
		t.Fatal("legacy result lost")
	}
	// Simulate an interrupted job whose exact-content result is already cached.
	if err = s.photos.SaveCaption(2, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.photos.AnalysisState(2, "running", "", false); err != nil {
		t.Fatal(err)
	}
	result, err = s.AnalyzePending()
	if err != nil || result.Done != 1 || result.Metrics.Cached != 1 || len(f.calls) != 1 {
		t.Fatalf("cache not reused: %+v %v", result, err)
	}
}

func TestTagQueueFatalLeavesUnfinishedJobsForResume(t *testing.T) {
	s, f := tagTestService(t, 20, false)
	f.fail = &gemini.APIError{Status: 403, Message: "denied"}
	result, err := s.AnalyzePending()
	if err != nil || result.Done != 0 || result.Failed == 0 {
		t.Fatalf("%+v %v", result, err)
	}
	pending, err := s.photos.PendingAnalysis(s.dest)
	if err != nil || len(pending) != 20 {
		t.Fatal("unfinished queue lost")
	}
	f.fail = nil
	result, err = s.AnalyzePending()
	if err != nil || result.Done != 20 {
		t.Fatalf("resume failed: %+v %v", result, err)
	}
}

func TestTagQueueCancelAndResume(t *testing.T) {
	s, f := tagTestService(t, 12, false)
	f.started = make(chan struct{}, 2)
	done := make(chan AnalyzeResult, 1)
	go func() { result, _ := s.AnalyzePending(); done <- result }()
	<-f.started
	s.CancelAnalyze()
	result := <-done
	if !result.Cancelled || result.Failed != 0 || result.Done != 0 {
		t.Fatalf("cancel marked photos failed: %+v", result)
	}
	f.started = nil
	result, err := s.AnalyzePending()
	if err != nil || result.Done != 12 {
		t.Fatalf("resume: %+v %v", result, err)
	}
}

func TestTagSampleLimitsWorkAndFullRunReusesIt(t *testing.T) {
	s, f := tagTestService(t, 105, true)
	result, err := s.AnalyzeSample()
	if err != nil || result.Done != 100 {
		t.Fatalf("sample: %+v %v", result, err)
	}
	result, err = s.AnalyzePending()
	if err != nil || result.Done != 5 || result.Metrics.Cached != 5 || len(f.calls) != 1 {
		t.Fatalf("full run repeated sample: %+v %v", result, err)
	}
}

func TestBatchCommitsEachResultBeforeWholeBatchFinishes(t *testing.T) {
	s, _ := tagTestService(t, 3, false)
	photos, err := s.photos.PendingAnalysis(s.dest)
	if err != nil {
		t.Fatal(err)
	}
	groups := []tagGroup{}
	for _, p := range photos {
		hash, err := analysisHash(context.Background(), p.BackupPath)
		if err != nil {
			t.Fatal(err)
		}
		groups = append(groups, tagGroup{hash: hash, photos: []model.Photo{p}})
	}
	out := make(chan tagOutcome) // Block the worker between individual results.
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(out)
		s.processTagGroups(context.Background(), s.dest, "incremental-test", groups, out)
	}()
	defer func() {
		for range out {
		}
		<-done
	}()
	first := <-out
	if first.err != nil {
		t.Fatal(first.err)
	}
	select {
	case <-done:
		t.Fatal("test requires the batch still running")
	default:
	}
	p, err := s.photos.PhotoByID(s.dest, first.photo.ID)
	if err != nil || p.Caption != "ภาพทดสอบสั้น" || len(p.Tags) != 5 || p.AnalysisState != "done" {
		t.Fatalf("success arrived before SQLite commit: %+v %v", p, err)
	}
	found, err := s.SearchPhotos("ทะเล", nil)
	if err != nil || len(found) == 0 {
		t.Fatalf("saved photo not searchable during batch: %v", err)
	}
}
