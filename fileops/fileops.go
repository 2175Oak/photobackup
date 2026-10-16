// Package fileops รวมฟังก์ชันจัดการไฟล์รูปบนดิสก์ (ใช้เฉพาะ standard library)
// แยกออกมาจาก service เพื่อให้ทดสอบได้โดยไม่ต้องพึ่งฐานข้อมูลหรือ Wails
package fileops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSourceNotRemoved หมายถึงคัดลอกไปปลายทางสำเร็จแล้ว แต่ลบไฟล์ต้นทางไม่ได้
// (เช่น ไฟล์ถูกเปิดค้างอยู่) ผู้เรียกควรถือว่าไฟล์อยู่ที่ปลายทางแล้ว
var ErrSourceNotRemoved = errors.New("คัดลอกแล้ว แต่ลบไฟล์ต้นทางไม่ได้")

var imageExts = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {},
	".webp": {}, ".bmp": {}, ".heic": {}, ".heif": {},
}

// IsImage ตรวจจากนามสกุลไฟล์ ไม่นับไฟล์ที่ขึ้นต้นด้วยจุด
// (เช่น ._IMG_0001.jpg ที่เครื่อง Mac สร้างทิ้งไว้ใน flash drive)
func IsImage(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") {
		return false
	}
	_, ok := imageExts[strings.ToLower(filepath.Ext(base))]
	return ok
}

// Info คือข้อมูลของไฟล์รูปหนึ่งไฟล์
type Info struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// ListImages คืนรายการไฟล์รูปในโฟลเดอร์ (ไม่รวมโฟลเดอร์ย่อย) เรียงตามชื่อ
func ListImages(dir string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !IsImage(e.Name()) {
			continue
		}
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, Info{Name: e.Name(), Size: fi.Size(), ModTime: fi.ModTime()})
	}
	return out, nil
}

// Exists เช็คว่ามีไฟล์ธรรมดา (ไม่ใช่โฟลเดอร์) อยู่จริงหรือไม่
func Exists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// NormalizePath ทำให้ path เป็นรูปแบบเดียวกันเสมอ (ตัดเครื่องหมายคำพูด, ตัด / ท้ายสุด, แปลงเป็น path เต็ม)
// ใช้ก่อนบันทึกและก่อนค้นหา เพื่อให้ C:\backup กับ C:\backup\ นับเป็นโฟลเดอร์เดียวกัน
func NormalizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"'`)
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("ยังไม่ได้ระบุโฟลเดอร์")
	}
	return filepath.Abs(filepath.Clean(p))
}

// SameDir เช็คว่า a กับ b คือโฟลเดอร์เดียวกันหรือไม่
func SameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(ai, bi)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// MoveFile ย้ายไฟล์ โดยลอง os.Rename ก่อน (เร็วที่สุดเมื่ออยู่ไดรฟ์เดียวกัน)
// ถ้าไม่สำเร็จ (เช่น ข้ามไดรฟ์) จะคัดลอกให้ครบก่อน ตรวจขนาด แล้วค่อยลบต้นทาง
// ไม่เช็ค error code เฉพาะ OS เพราะ Windows/macOS/Linux คืนค่าไม่เหมือนกัน
func MoveFile(src, dst string) error {
	renameErr := os.Rename(src, dst)
	if renameErr == nil {
		return nil
	}

	if err := CopyFile(src, dst); err != nil {
		return fmt.Errorf("คัดลอกไม่สำเร็จ (rename: %v): %w", renameErr, err)
	}

	si, errS := os.Stat(src)
	di, errD := os.Stat(dst)
	if errS != nil || errD != nil || si.Size() != di.Size() {
		_ = os.Remove(dst)
		return errors.New("ขนาดไฟล์ปลายทางไม่ตรงกับต้นทาง จึงยกเลิกการย้าย")
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("%w: %v", ErrSourceNotRemoved, err)
	}
	return nil
}

// CopyFile คัดลอกด้วย buffer 1MB เขียนลงไฟล์ชั่วคราว (.part) ก่อน sync แล้วค่อยเปลี่ยนชื่อ
// ทำให้ถ้าถอด flash drive กลางทาง จะไม่เหลือไฟล์ปลายทางที่ไม่สมบูรณ์ให้ถูกนับว่า "ซ้ำ"
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	si, err := in.Stat()
	if err != nil {
		return err
	}

	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	buf := make([]byte, 1024*1024)
	if _, err := io.CopyBuffer(out, in, buf); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	// เก็บวันเวลาแก้ไขเดิมของรูปไว้ (ไม่สำคัญมาก ถ้าตั้งไม่ได้ก็ข้าม)
	_ = os.Chtimes(dst, time.Now(), si.ModTime())
	return nil
}
