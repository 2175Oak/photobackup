package repository

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"photo-backup/model"
	"testing"
)

func TestSearchTyposAndRanking(t *testing.T) {
	photos := []model.Photo{
		{ID: 3, Caption: "สุนัขนอนข้างหน้าต่าง", Tags: []model.Tag{{Name: "สัตว์"}}},
		{ID: 2, Caption: "วิวริมทะเล", Tags: []model.Tag{{Name: "ทะเล"}}},
		{ID: 1, Caption: "สุนัขวิ่งเล่น", Tags: []model.Tag{{Name: "สุนัข"}, {Name: "กลางแจ้ง"}}},
	}
	for _, q := range []string{"สุนัก", "สุนข", "สนุัข"} {
		result := rankPhotos(photos, []string{q})
		if len(result) != 2 || result[0].ID != 1 {
			t.Fatalf("%q: %+v", q, result)
		}
	}
	exact := rankPhotos([]model.Photo{{ID: 1, Tags: []model.Tag{{Name: "coat"}}}, {ID: 2, Caption: "A cat sleeping"}}, []string{"CAT"})
	if len(exact) != 2 || exact[0].ID != 2 {
		t.Fatalf("exact must precede fuzzy: %+v", exact)
	}
	result := rankPhotos(photos, []string{"สุนัก", "กลางแจง"})
	if len(result) != 1 || result[0].ID != 1 {
		t.Fatalf("multiword: %+v", result)
	}
	if len(rankPhotos(photos, []string{"เครื่องบิน"})) != 0 {
		t.Fatal("unrelated result returned")
	}
	if len(rankPhotos([]model.Photo{{Caption: "dog"}}, []string{"dt"})) != 0 {
		t.Fatal("short queries must not match fuzzily")
	}
}

func TestSubstringEdits(t *testing.T) {
	for _, tc := range []struct {
		q, text  string
		distance int
	}{
		{"สุนัก", "รูปสุนัขนอนบนเตียง", 1}, {"cat", "a coat on a chair", 1},
		{"taht", "that", 1}, {"ทะเล", "ทะเล", 0}, {"ทะเล", "", 4},
		{"สุนข", "สุนัข", 1},
	} {
		if got := substringEdits([]rune(tc.q), []rune(tc.text)); got != tc.distance {
			t.Errorf("%q/%q=%d want %d", tc.q, tc.text, got, tc.distance)
		}
	}
}

func TestSearchSQLiteKeepsDestinationAndSelectedTagFilters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "search.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err = db.AutoMigrate(&model.Photo{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	r := NewPhotoRepository(db)
	for _, p := range []model.Photo{{ID: 1, Dest: "a", FileName: "1.jpg", BackupPath: "a/1.jpg"}, {ID: 2, Dest: "a", FileName: "2.jpg", BackupPath: "a/2.jpg"}, {ID: 3, Dest: "b", FileName: "3.jpg", BackupPath: "b/3.jpg"}} {
		if err = r.Create(&p); err != nil {
			t.Fatal(err)
		}
		tags := []string{"สุนัข"}
		if p.ID != 2 {
			tags = append(tags, "บ้าน")
		}
		if err = r.SaveAnalysis(p.ID, "", tags); err != nil {
			t.Fatal(err)
		}
	}
	result, err := r.Search("a", []string{"สุนัก"}, []string{"บ้าน"})
	if err != nil || len(result) != 1 || result[0].ID != 1 {
		t.Fatalf("filter leakage: %+v %v", result, err)
	}
	result, err = r.Search("a", nil, nil)
	if err != nil || len(result) != 2 || result[0].ID != 2 {
		t.Fatalf("empty query order: %+v %v", result, err)
	}
	result, err = r.Search("a", []string{"%"}, nil)
	if err != nil || len(result) != 0 {
		t.Fatal("query interpreted as SQL wildcard")
	}
}

func BenchmarkSearch1000Photos(b *testing.B) {
	photos := make([]model.Photo, 1000)
	for i := range photos {
		photos[i] = model.Photo{ID: int64(i), Caption: "สุนัขนอนบนโซฟาข้างหน้าต่างในห้องรับแขก", Tags: []model.Tag{{Name: "สุนัข"}, {Name: "บ้าน"}, {Name: "โซฟา"}}}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rankPhotos(photos, []string{"สุนัก", "โซฟา"})
	}
}
