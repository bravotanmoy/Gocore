package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var allowedUploadExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".csv": true,
	".mp4": true, ".mp3": true, ".zip": true, ".txt": true,
}

// saveUpload stores the "file" form field on disk and records it; returns an error message on failure.
func (h *Handler) saveUpload(c *gin.Context) (*models.Media, string) {
	file, err := c.FormFile("file")
	if err != nil {
		return nil, "no file uploaded (use multipart field \"file\")"
	}
	if file.Size > h.Cfg.MaxUploadMB*1024*1024 {
		return nil, fmt.Sprintf("file exceeds %d MB limit", h.Cfg.MaxUploadMB)
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedUploadExt[ext] {
		return nil, "file type not allowed: " + ext
	}

	sub := time.Now().Format("2006/01")
	dir := filepath.Join(h.Cfg.UploadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "could not create upload directory"
	}
	name := uuid.NewString() + ext
	if err := c.SaveUploadedFile(file, filepath.Join(dir, name)); err != nil {
		return nil, "could not save file"
	}

	user := middleware.CurrentUser(c)
	media := models.Media{
		FileName:   file.Filename,
		Path:       "/uploads/" + sub + "/" + name,
		MimeType:   file.Header.Get("Content-Type"),
		Size:       file.Size,
		UploadedBy: user.ID,
	}
	if err := database.DB.Create(&media).Error; err != nil {
		return nil, "could not record upload"
	}
	return &media, ""
}

// removeUploadFile deletes the physical file behind a media path.
func removeUploadFile(uploadDir, path string) {
	rel := strings.TrimPrefix(path, "/uploads/")
	_ = os.Remove(filepath.Join(uploadDir, filepath.FromSlash(rel)))
}

// UploadMedia handles POST /api/v1/media (multipart field "file").
func (h *Handler) UploadMedia(c *gin.Context) {
	media, errMsg := h.saveUpload(c)
	if errMsg != "" {
		utils.Fail(c, http.StatusBadRequest, errMsg)
		return
	}
	c.Set("activity", "uploaded file "+media.FileName)
	c.Set("activity_entity", "media")
	c.Set("activity_entity_id", strconv.Itoa(int(media.ID)))
	utils.Created(c, media)
}

// ListMedia handles GET /api/v1/media
func (h *Handler) ListMedia(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Media{})
	if s := c.Query("search"); s != "" {
		q = q.Where("file_name LIKE ?", "%"+s+"%")
	}
	var total int64
	q.Count(&total)
	var items []models.Media
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&items)
	utils.Paginated(c, items, total, page, perPage)
}

// DeleteMedia handles DELETE /api/v1/media/:id — removes DB row and file on disk.
func (h *Handler) DeleteMedia(c *gin.Context) {
	var media models.Media
	if err := database.DB.First(&media, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "media not found")
		return
	}
	removeUploadFile(h.Cfg.UploadDir, media.Path)
	database.DB.Delete(&media)
	c.Set("activity", "deleted file "+media.FileName)
	c.Set("activity_entity", "media")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "media deleted"})
}
