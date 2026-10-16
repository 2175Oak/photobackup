package service

import (
	"os"
	"path/filepath"

	"photo-backup/fileops"
)

// CheckIntegrity ตรวจว่ารายการในฐานข้อมูลของปลายทางปัจจุบันยังมีไฟล์จริงครบหรือไม่ (เทียบเท่า /check)
func (s *PhotoService) CheckIntegrity() ([]IntegrityItem, error) {
	dest := s.snapshot().Dest
	if dest == "" {
		return nil, errNoDest
	}
	return s.checkIntegrity(dest)
}

func (s *PhotoService) checkIntegrity(dest string) ([]IntegrityItem, error) {
	list, err := s.photos.ListByDest(dest)
	if err != nil {
		return nil, err
	}
	items := make([]IntegrityItem, 0, len(list))
	for _, p := range list {
		items = append(items, IntegrityItem{
			Filename: p.FileName,
			OK:       fileops.Exists(filepath.Join(dest, p.FileName)),
		})
	}
	return items, nil
}

// Delete ลบรูปที่ระบุชื่อออกจากปลายทาง ทั้งไฟล์จริงและประวัติ (เทียบเท่า /delete file1,file2)
func (s *PhotoService) Delete(names []string) (DeleteResult, error) {
	return s.deleteFromDest(names)
}

// DeleteAll ลบรูปทั้งหมดในปลายทาง ทั้งไฟล์จริงและประวัติ (เทียบเท่า /delete all)
func (s *PhotoService) DeleteAll() (DeleteResult, error) {
	dest := s.snapshot().Dest
	if dest == "" {
		return DeleteResult{}, errNoDest
	}
	imgs, err := fileops.ListImages(dest)
	if err != nil {
		return DeleteResult{}, err
	}
	names := make([]string, 0, len(imgs))
	for _, f := range imgs {
		names = append(names, f.Name)
	}
	return s.deleteFromDest(names)
}

func (s *PhotoService) deleteFromDest(names []string) (DeleteResult, error) {
	if !s.opMu.TryLock() {
		return DeleteResult{}, errBusy
	}
	defer s.opMu.Unlock()

	dest := s.snapshot().Dest
	if dest == "" {
		return DeleteResult{}, errNoDest
	}

	res := DeleteResult{Items: make([]FileResult, 0, len(names))}
	for _, name := range names {
		item := FileResult{Name: name}
		full := filepath.Join(dest, name)

		switch {
		case name == "" || name != filepath.Base(name) || !fileops.IsImage(name):
			item.Status = StatusFailed
			item.Message = "ชื่อไฟล์ไม่ถูกต้อง หรือไม่ใช่ไฟล์รูป"
		case !fileops.Exists(full):
			item.Status = StatusFailed
			item.Message = "ไม่พบไฟล์ในปลายทาง"
		default:
			// ลบไฟล์จริงก่อนเสมอ ถ้าลบไม่สำเร็จจะไม่แตะฐานข้อมูล
			if err := os.Remove(full); err != nil {
				item.Status = StatusFailed
				item.Message = "ลบไฟล์ไม่สำเร็จ: " + err.Error()
				break
			}
			item.Status = StatusDeleted
			n, err := s.photos.DeleteByDestAndName(dest, name)
			switch {
			case err != nil:
				item.Message = "ลบไฟล์แล้ว แต่ลบประวัติในฐานข้อมูลไม่สำเร็จ: " + err.Error()
			case n == 0:
				item.Message = "ลบไฟล์แล้ว (ไม่มีประวัติของไฟล์นี้ในฐานข้อมูล)"
			}
		}

		if item.Status == StatusDeleted {
			res.Deleted++
		} else {
			res.Failed++
		}
		res.Items = append(res.Items, item)
	}
	return res, nil
}

// CleanDest ล้างประวัติของปลายทางปัจจุบันในฐานข้อมูล ไม่ลบไฟล์รูปจริง (เทียบเท่า /clean)
func (s *PhotoService) CleanDest() (int64, error) {
	if !s.opMu.TryLock() {
		return 0, errBusy
	}
	defer s.opMu.Unlock()

	dest := s.snapshot().Dest
	if dest == "" {
		return 0, errNoDest
	}
	return s.photos.DeleteByDest(dest)
}

// CleanAll รีเซ็ตฐานข้อมูลทั้งหมด ทั้งประวัติ แท็ก และโฟลเดอร์ที่จำไว้ ไม่ลบไฟล์รูปจริง (เทียบเท่า /clean all)
func (s *PhotoService) CleanAll() (CleanAllResult, error) {
	if !s.opMu.TryLock() {
		return CleanAllResult{}, errBusy
	}
	defer s.opMu.Unlock()

	photos, tags, err := s.photos.DeleteAll()
	if err != nil {
		return CleanAllResult{}, err
	}
	settings, err := s.settings.DeleteAll()
	if err != nil {
		return CleanAllResult{}, err
	}

	s.mu.Lock()
	s.source, s.dest = "", ""
	s.mu.Unlock()

	return CleanAllResult{Photos: photos, Tags: tags, Settings: settings}, nil
}
