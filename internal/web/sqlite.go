// Lab 7: Implement a SQLite video metadata service

package web

import (
	"database/sql"
	"errors"
	"time"
	"os"
	"log"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct{
	db *sql.DB
}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)

func NewSQLiteVideoMetadataService(path string) (*SQLiteVideoMetadataService, error) {
	_, err := os.Stat(path)
	dbExist := !errors.Is(err, os.ErrNotExist)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalf("failed to open database %s\n", err)
		return nil, err
	}

	if !dbExist {
		createTableStmt := `
		CREATE TABLE IF NOT EXISTS videos (
			video_id TEXT PRIMARY KEY,
			uploaded_at TIMESTAMP
		);`
		if _, err := db.Exec(createTableStmt); err != nil {
			db.Close()
			log.Fatalf("failed to create table %s\n", err)
			return nil, err
		}
	}
	return &SQLiteVideoMetadataService{db: db}, nil
}


func (s *SQLiteVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	_, err := s.db.Exec("INSERT INTO videos (video_id, uploaded_at) VALUES (?, ?)", videoId, uploadedAt)
	if err != nil {
		// if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
		// 	return errors.New("video ID already exists")
		// }
		log.Fatalf("Failed to insert %s\n", err)
		return err
	}
	return nil
}

func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	rows, err := s.db.Query("SELECT video_id, uploaded_at FROM videos ORDER BY uploaded_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []VideoMetadata
	for rows.Next() {
		var vm VideoMetadata
		if err := rows.Scan(&vm.Id, &vm.UploadedAt); err != nil {
			return nil, err
		}
		list = append(list, vm)
	}
	return list, nil
}

func (s *SQLiteVideoMetadataService) Read(videoId string) (*VideoMetadata, error) {
	row := s.db.QueryRow("SELECT video_id, uploaded_at FROM videos WHERE video_id = ?", videoId)

	var vm VideoMetadata
	if err := row.Scan(&vm.Id, &vm.UploadedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &vm, nil
}
