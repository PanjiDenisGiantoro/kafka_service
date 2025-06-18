package model

// People adalah struktur model untuk tabel "people"
type People struct {
	ID   int    `gorm:"primary_key"`
	Name string `gorm:"type:varchar(100)"`
	Age  int
	City string
}
