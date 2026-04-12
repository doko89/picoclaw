package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCTaskImageRoutes binds task image endpoints to the ServeMux.
func (h *Handler) registerMCTaskImageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/images", h.handleListMCTaskImages)
	mux.HandleFunc("POST /api/mc/tasks/{id}/images", h.handleUploadMCTaskImage)
	mux.HandleFunc("DELETE /api/mc/tasks/{id}/images", h.handleDeleteMCTaskImage)
	mux.HandleFunc("GET /api/mc/task-images/{taskId}/{filename}", h.handleServeMCTaskImage)
}

// allowedImageTypes is the set of allowed MIME types for image uploads.
var allowedImageTypes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

// maxImageSize is 10MB.
const maxImageSize = 10 * 1024 * 1024

// taskImageDir returns the directory for task images under picoHome.
func taskImageDir(picoHome, taskID string) string {
	return filepath.Join(picoHome, "mc-data", "task-images", taskID)
}

type mcTaskImage struct {
	Filename     string `json:"filename"`
	OriginalName string `json:"original_name"`
	UploadedAt   string `json:"uploaded_at"`
}

// handleListMCTaskImages returns all images attached to a task.
//
//	GET /api/mc/tasks/{id}/images
func (h *Handler) handleListMCTaskImages(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var imagesJSON *string
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT images FROM tasks WHERE id = ?", taskID,
	).Scan(&imagesJSON)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("list images: %v", err))
		http.Error(w, "Failed to list images", http.StatusInternalServerError)
		return
	}

	var images []mcTaskImage
	if imagesJSON != nil && *imagesJSON != "" {
		_ = json.Unmarshal([]byte(*imagesJSON), &images)
	}
	if images == nil {
		images = []mcTaskImage{}
	}
	writeJSON(w, map[string]any{"images": images})
}

// handleUploadMCTaskImage uploads an image for a task.
//
//	POST /api/mc/tasks/{id}/images
func (h *Handler) handleUploadMCTaskImage(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize)

	err := r.ParseMultipartForm(maxImageSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !allowedImageTypes[contentType] {
		http.Error(w, fmt.Sprintf("File type not allowed: %s", contentType), http.StatusBadRequest)
		return
	}

	// Create task image directory
	dir := taskImageDir(h.picoHome, taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.ErrorC("mc", fmt.Sprintf("upload image - mkdir: %v", err))
		http.Error(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}

	// Sanitize filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, header.Filename)
	filename := fmt.Sprintf("%d-%s", 0, safeName)

	// Create destination file
	dstPath := filepath.Join(dir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("upload image - create file: %v", err))
		http.Error(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		logger.ErrorC("mc", fmt.Sprintf("upload image - write: %v", err))
		http.Error(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}

	// Update task images JSON
	newImage := mcTaskImage{
		Filename:     filename,
		OriginalName: header.Filename,
		UploadedAt:   "", // DB will handle timestamp via updated_at
	}

	var imagesJSON *string
	h.mcDB.QueryRowContext(r.Context(),
		"SELECT images FROM tasks WHERE id = ?", taskID,
	).Scan(&imagesJSON)

	var images []mcTaskImage
	if imagesJSON != nil && *imagesJSON != "" {
		_ = json.Unmarshal([]byte(*imagesJSON), &images)
	}
	images = append(images, newImage)
	updatedJSON, _ := json.Marshal(images)

	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET images = ?, updated_at = datetime('now') WHERE id = ?",
		string(updatedJSON), taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("upload image - update db: %v", err))
		http.Error(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"image": newImage,
		"total": len(images),
	})
}

// handleDeleteMCTaskImage removes an image from a task.
//
//	DELETE /api/mc/tasks/{id}/images
func (h *Handler) handleDeleteMCTaskImage(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	// Remove from images JSON
	var imagesJSON *string
	h.mcDB.QueryRowContext(r.Context(),
		"SELECT images FROM tasks WHERE id = ?", taskID,
	).Scan(&imagesJSON)

	var images []mcTaskImage
	if imagesJSON != nil && *imagesJSON != "" {
		_ = json.Unmarshal([]byte(*imagesJSON), &images)
	}

	var filtered []mcTaskImage
	for _, img := range images {
		if img.Filename != input.Filename {
			filtered = append(filtered, img)
		}
	}
	if filtered == nil {
		filtered = []mcTaskImage{}
	}

	updatedJSON, _ := json.Marshal(filtered)
	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET images = ?, updated_at = datetime('now') WHERE id = ?",
		string(updatedJSON), taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete image - update db: %v", err))
		http.Error(w, "Failed to delete image", http.StatusInternalServerError)
		return
	}

	// Best-effort file deletion
	dir := taskImageDir(h.picoHome, taskID)
	_ = os.Remove(filepath.Join(dir, input.Filename))

	writeJSON(w, map[string]any{
		"success":   true,
		"remaining": len(filtered),
	})
}

// handleServeMCTaskImage serves a task image file.
//
//	GET /api/mc/task-images/{taskId}/{filename}
func (h *Handler) handleServeMCTaskImage(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	filename := r.PathValue("filename")

	// Prevent path traversal
	if strings.Contains(taskID, "..") || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(taskImageDir(h.picoHome, taskID), filename)
	http.ServeFile(w, r, filePath)
}