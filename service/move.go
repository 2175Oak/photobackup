package service

import (
	"errors"
	"path/filepath"
	"time"

	"photo-backup/fileops"
	"photo-backup/model"
)

// Move ย้ายรูปที่ระบุชื่อจากต้นทางไปปลายทาง (เทียบเท่า /move file1,file2)
func (s *PhotoService) Move(names []string) (MoveResult, error) {
	return s.move(names)
}

// MoveAll ย้ายรูปทั้งหมดในต้นทางไปปลายทาง (เทียบเท่า /move all)
func (s *PhotoService) MoveAll() (MoveResult, error) {
	src := s.snapshot().Source
	if src == "" {
		return MoveResult{}, errNoSource
	}
	imgs, err := fileops.ListImages(src)
	if err != nil {
		return MoveResult{}, err
	}
	names := make([]string, 0, len(imgs))
	for _, f := range imgs {
		names = append(names, f.Name)
	}
	return s.move(names)
}

func (s *PhotoService) move(names []string) (MoveResult, error) {
	if !s.opMu.TryLock() {
		return MoveResult{}, errBusy
	}
	defer s.opMu.Unlock()

	st := s.snapshot()
	if st.Source == "" {
		return MoveResult{}, errNoSource
	}
	if st.Dest == "" {
		return MoveResult{}, errNoDest
	}
	if len(names) == 0 {
		return MoveResult{}, errors.New("ไม่มีรูปให้ย้าย")
	}

	start := time.Now()

	// ดึงรายชื่อที่มีในฐานข้อมูลของปลายทางนี้มาครั้งเดียว แทนการ query ทีละไฟล์
	existing, err := s.photos.NamesByDest(st.Dest)
	if err != nil {
		return MoveResult{}, err
	}

	items := make([]FileResult, 0, len(names))
	var records []model.Photo
	var recordItem []int // recordItem[i] = ตำแหน่งใน items ของ records[i]

	for i, name := range names {
		item, rec := s.moveOne(st, name, existing)
		if rec != nil {
			records = append(records, *rec)
			recordItem = append(recordItem, len(items))
		}
		items = append(items, item)
		s.emit("move:progress", ProgressEvent{Done: i + 1, Total: len(names), Name: name, Status: item.Status})
	}

	// บันทึกประวัติรวดเดียวหลังย้ายครบ ถ้าบันทึกรวดเดียวไม่ผ่าน ให้ลองทีละรายการ
	// เพื่อไม่ให้มีรูปที่ย้ายไปแล้วโดยไม่มีประวัติ และบอกผู้ใช้ชัดเจนถ้ายังบันทึกไม่ได้
	if len(records) > 0 {
		if err := s.photos.CreateBatch(records); err != nil {
			for i := range records {
				if err := s.photos.Create(&records[i]); err != nil {
					items[recordItem[i]].Status = StatusFailed
					items[recordItem[i]].Message = "ย้ายแล้ว แต่บันทึกประวัติลงฐานข้อมูลไม่สำเร็จ: " + err.Error()
				}
			}
		}
	}

	res := MoveResult{Items: items, Seconds: time.Since(start).Seconds()}
	for _, it := range items {
		switch it.Status {
		case StatusMoved:
			res.Moved++
		case StatusSkipped:
			res.Skipped++
		default:
			res.Failed++
		}
	}
	return res, nil
}

// moveOne ย้ายไฟล์เดียว ถ้าสำเร็จจะคืน record ที่ต้องบันทึกลงฐานข้อมูลด้วย
func (s *PhotoService) moveOne(st State, name string, existing map[string]bool) (FileResult, *model.Photo) {
	item := FileResult{Name: name}

	// ชื่อไฟล์ต้องเป็นชื่อล้วนๆ ไม่มี path และต้องเป็นไฟล์รูป
	if name == "" || name != filepath.Base(name) || !fileops.IsImage(name) {
		item.Status = StatusFailed
		item.Message = "ชื่อไฟล์ไม่ถูกต้อง หรือไม่ใช่ไฟล์รูป"
		return item, nil
	}

	srcPath := filepath.Join(st.Source, name)
	dstPath := filepath.Join(st.Dest, name)

	// ตรวจไฟล์ซ้ำทั้งในฐานข้อมูลและในโฟลเดอร์ปลายทางจริง
	if existing[name] || fileops.Exists(dstPath) {
		item.Status = StatusSkipped
		item.Message = "มีรูปชื่อนี้ในปลายทางหรือฐานข้อมูลแล้ว"
		return item, nil
	}
	if !fileops.Exists(srcPath) {
		item.Status = StatusFailed
		item.Message = "ไม่พบไฟล์ในต้นทาง"
		return item, nil
	}

	if err := fileops.MoveFile(srcPath, dstPath); err != nil {
		// คัดลอกสำเร็จแต่ลบต้นทางไม่ได้ ไฟล์อยู่ที่ปลายทางแล้ว จึงต้องบันทึกประวัติด้วย
		if !errors.Is(err, fileops.ErrSourceNotRemoved) {
			item.Status = StatusFailed
			item.Message = err.Error()
			return item, nil
		}
		item.Message = err.Error()
	}

	item.Status = StatusMoved
	existing[name] = true // กันชื่อซ้ำที่อยู่ในรายการเดียวกันหลุดผ่านมาอีก
	return item, &model.Photo{FileName: name, Dest: st.Dest, BackupPath: dstPath}
}
