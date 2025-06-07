// Lab 7: Implement a web server

package web

import (
	"bytes"
	// "fmt"
	// "html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {
	return &server{
		metadataService: metadataService,
		contentService:  contentService,
	}
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/", s.handleIndex)
	return http.Serve(lis, s.mux)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// panic("Lab 7: not implemented")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	videos, err := s.metadataService.List()
	if err != nil {
		http.Error(w, "Failed to list videos", http.StatusInternalServerError)
		return
	}
	type VideoEntry struct {
		Id        string
		EscapedId string
		UploadTime      string
	}
	var entries []VideoEntry
	for _, v := range videos {
		entries = append(entries, VideoEntry{
			Id:        v.Id,
			EscapedId: url.PathEscape(v.Id),
			UploadTime:      v.UploadedAt.Format(time.RFC1123),
		})
	}
	indexTmpl.Execute(w, entries)

}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// panic("Lab 7: not implemented")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := header.Filename
	if !strings.HasSuffix(filename, ".mp4") {
		http.Error(w, "Only .mp4 files are allowed", http.StatusBadRequest)
		return
	}

	videoID := strings.TrimSuffix(filename, ".mp4")
	if existing, _ := s.metadataService.Read(videoID); existing != nil {
		http.Error(w, "Video ID already exists", http.StatusConflict)
		return
	}

	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		http.Error(w, "Failed to create temp dir", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, filename)
	outFile, err := os.Create(videoPath)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, file); err != nil {
		http.Error(w, "Failed to write uploaded file", http.StatusInternalServerError)
		return
	}

	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-bf", "1",
		"-keyint_min", "120",
		"-g", "120",
		"-sc_threshold", "0",
		"-b:v", "3000k",
		"-b:a", "128k",
		"-f", "dash",
		"-use_timeline", "1",
		"-use_template", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-seg_duration", "4",
		manifestPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Println("ffmpeg error:", stderr.String())
		http.Error(w, "FFmpeg failed", http.StatusInternalServerError)
		return
	}

	// Save all files from tempDir
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relName := filepath.Base(path)
	
		if strings.HasSuffix(relName, ".mpd") || strings.HasSuffix(relName, ".m4s") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return s.contentService.Write(videoID, relName, data)
		}
	
		return nil
	})
	if err != nil {
		http.Error(w, "Failed to save content", http.StatusInternalServerError)
		return
	}

	if err := s.metadataService.Create(videoID, time.Now()); err != nil {
		http.Error(w, "Failed to save metadata", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)


}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	log.Println("Video ID:", videoId)

	video, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if video == nil {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}
	type VideoPageData struct {
		Id         string
		UploadedAt string
	}
	data := VideoPageData{
		Id:         video.Id,
		UploadedAt: video.UploadedAt.Format("2006-01-02 15:04:05"),
	}

	videoTmpl.Execute(w, data)
}

func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	// parse /content/<videoId>/<filename>
	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid content path", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]
	log.Println("Video ID:", videoId, "Filename:", filename)
	// panic("Lab 7: not implemented")
	data, err := s.contentService.Read(videoId, filename)
	if err != nil {
		http.Error(w, "Failed to read content", http.StatusInternalServerError)
		return
	}
	w.Write(data) 
}
