package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/store"
	"github.com/0x2E/fusion/internal/translation"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

type translationJob struct {
	State  string
	Error  string
	Cancel context.CancelFunc
}

type translationResponse struct {
	Segments  []translation.Segment `json:"segments"`
	State     string                `json:"state"`
	Error     string                `json:"error,omitempty"`
	Model     string                `json:"model"`
	Available bool                  `json:"available"`
}

func (h *Handler) translationSourceKey(item *model.Item) string {
	key := sha256.Sum256([]byte("paragraph-text-v1\x00" + strconv.FormatInt(item.ID, 10) + "\x00" + h.config.TranslationModel + "\x00" + item.Title + "\x00" + item.Content))
	return hex.EncodeToString(key[:])
}

func (h *Handler) translationItem(c *gin.Context) (*model.Item, string, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		badRequestError(c, "invalid id")
		return nil, "", false
	}
	item, err := h.store.GetItem(id)
	if errors.Is(err, store.ErrNotFound) {
		notFoundError(c, "item")
		return nil, "", false
	}
	if err != nil {
		internalError(c, err, "get translation item")
		return nil, "", false
	}
	return item, h.translationSourceKey(item), true
}

func (h *Handler) translationStatus(itemID int64, sourceKey string) (translationResponse, error) {
	_, binaryErr := exec.LookPath(h.config.TranslationCodexPath)
	response := translationResponse{State: "idle", Model: h.config.TranslationModel, Available: binaryErr == nil}
	// Read job state before the cache: an idle response must include the last saved batch.
	h.translationMu.Lock()
	if job := h.translationJobs[sourceKey]; job != nil {
		response.State, response.Error = job.State, job.Error
	}
	h.translationMu.Unlock()
	segments, err := h.store.GetTranslation(itemID, sourceKey)
	if err != nil {
		return translationResponse{}, err
	}
	response.Segments = segments
	return response, nil
}

func (h *Handler) getTranslation(c *gin.Context) {
	item, key, ok := h.translationItem(c)
	if !ok {
		return
	}
	response, err := h.translationStatus(item.ID, key)
	if err != nil {
		internalError(c, err, "get translation")
		return
	}
	dataResponse(c, response)
}

func normalizeTranslationText(value string) string { return strings.Join(strings.Fields(value), " ") }

func validateTranslationSegments(content string, segments []translation.Segment) bool {
	if len(segments) == 0 || len(segments) > 2000 {
		return false
	}
	document, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return false
	}
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	original := normalizeTranslationText(text.String())
	seen := map[string]bool{}
	size := 0
	for _, segment := range segments {
		size += len(segment.Text)
		if segment.ID == "" || len(segment.ID) > 40 || seen[segment.ID] || len(segment.Text) > 64000 ||
			normalizeTranslationText(segment.Text) == "" || !strings.Contains(original, normalizeTranslationText(segment.Text)) {
			return false
		}
		seen[segment.ID] = true
	}
	return size <= 1000000
}

func (h *Handler) startTranslation(c *gin.Context) {
	item, key, ok := h.translationItem(c)
	if !ok {
		return
	}
	var request struct {
		Segments []translation.Segment `json:"segments"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	if c.ShouldBindJSON(&request) != nil || !validateTranslationSegments(item.Content, request.Segments) {
		badRequestError(c, "invalid_translation_segments")
		return
	}
	if _, err := exec.LookPath(h.config.TranslationCodexPath); err != nil {
		c.JSON(503, gin.H{"error": "translation_unavailable"})
		return
	}
	cached, err := h.store.GetTranslation(item.ID, key)
	if err != nil {
		internalError(c, err, "get translation cache")
		return
	}
	existing := map[string]translation.Segment{}
	for _, segment := range cached {
		existing[segment.ID] = segment
	}
	missing := []translation.Segment{}
	for _, segment := range request.Segments {
		if previous, found := existing[segment.ID]; !found || previous.Text != segment.Text {
			missing = append(missing, translation.Segment{ID: segment.ID, Text: segment.Text})
		}
	}
	if len(missing) == 0 {
		dataResponse(c, translationResponse{Segments: cached, State: "idle", Model: h.config.TranslationModel, Available: true})
		return
	}
	h.translationMu.Lock()
	if job := h.translationJobs[key]; job != nil && job.State == "running" {
		h.translationMu.Unlock()
		c.JSON(202, gin.H{"data": translationResponse{Segments: cached, State: "running", Model: h.config.TranslationModel, Available: true}})
		return
	}
	if h.translationRunning {
		h.translationMu.Unlock()
		c.JSON(409, gin.H{"error": "translation_busy"})
		return
	}
	if h.translationJobs == nil {
		h.translationJobs = map[string]*translationJob{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	job := &translationJob{State: "running", Cancel: cancel}
	h.translationJobs[key] = job
	h.translationRunning = true
	h.translationMu.Unlock()
	go h.runTranslation(ctx, cancel, job, item, key, cached, missing)
	c.JSON(202, gin.H{"data": translationResponse{Segments: cached, State: "running", Model: h.config.TranslationModel, Available: true}})
}

func (h *Handler) runTranslation(ctx context.Context, cancel context.CancelFunc, job *translationJob, item *model.Item, key string, cached, missing []translation.Segment) {
	defer cancel()
	var failure error
	defer func() {
		h.translationMu.Lock()
		job.State = "idle"
		if failure != nil {
			job.State = "error"
			job.Error = "translation_failed"
		}
		h.translationRunning = false
		h.translationMu.Unlock()
		if failure != nil {
			slog.Warn("article translation failed", "item_id", item.ID, "error", failure)
		}
	}()
	runner := h.translateSegments
	if runner == nil {
		runner = translation.Translate
	}
	for len(missing) > 0 {
		batch := translation.Batch(missing)
		translated, err := runner(ctx, h.config.TranslationCodexPath, h.config.TranslationModel, item.Title, batch)
		if err != nil {
			failure = err
			return
		}
		for _, result := range translated {
			replaced := false
			for i := range cached {
				if cached[i].ID == result.ID {
					cached[i] = result
					replaced = true
					break
				}
			}
			if !replaced {
				cached = append(cached, result)
			}
		}
		if err := h.store.SaveTranslation(item.ID, key, cached); err != nil {
			failure = err
			return
		}
		missing = missing[len(batch):]
	}
	slog.Info("article translation completed", "item_id", item.ID, "segments", len(cached))
}

func (h *Handler) CloseTranslations() {
	h.translationMu.Lock()
	defer h.translationMu.Unlock()
	for _, job := range h.translationJobs {
		if job.Cancel != nil {
			job.Cancel()
		}
	}
}
