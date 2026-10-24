package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/0x2E/fusion/internal/fulltext"
	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/store"
	"github.com/gin-gonic/gin"
)

type fullContentJob struct {
	State  string
	Error  string
	Cancel context.CancelFunc
}
type fullContentResponse struct {
	State string      `json:"state"`
	Error string      `json:"error,omitempty"`
	Item  *model.Item `json:"item,omitempty"`
}

func (h *Handler) fullContentItem(c *gin.Context) (*model.Item, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		badRequestError(c, "invalid id")
		return nil, false
	}
	item, err := h.store.GetItem(id)
	if errors.Is(err, store.ErrNotFound) {
		notFoundError(c, "item")
		return nil, false
	}
	if err != nil {
		internalError(c, err, "get full content item")
		return nil, false
	}
	return item, true
}
func (h *Handler) fullContentStatus(item *model.Item) (fullContentResponse, error) {
	response := fullContentResponse{State: "idle"}
	h.fullContentMu.Lock()
	if job := h.fullContentJobs[item.ID]; job != nil {
		response.State = job.State
		response.Error = job.Error
	}
	h.fullContentMu.Unlock()
	// Read the cache after job state, so an idle result includes the final body.
	latest, err := h.store.GetItem(item.ID)
	if err != nil {
		return response, err
	}
	if latest.FullContentFetchedAt > 0 {
		response.State = "idle"
		response.Error = ""
		response.Item = latest
	}
	return response, nil
}
func (h *Handler) getFullContent(c *gin.Context) {
	item, ok := h.fullContentItem(c)
	if !ok {
		return
	}
	response, err := h.fullContentStatus(item)
	if err != nil {
		internalError(c, err, "get full content status")
		return
	}
	dataResponse(c, response)
}
func (h *Handler) startFullContent(c *gin.Context) {
	item, ok := h.fullContentItem(c)
	if !ok {
		return
	}
	if item.FullContentFetchedAt > 0 {
		dataResponse(c, fullContentResponse{State: "idle", Item: item})
		return
	}
	feed, err := h.store.GetFeed(item.FeedID)
	if err != nil {
		internalError(c, err, "get full content feed")
		return
	}
	if !feed.FetchFullContent {
		badRequestError(c, "full content is disabled for this feed")
		return
	}
	if !fulltext.SupportedURL(item.Link) {
		badRequestError(c, "full content currently supports OpenAI articles only")
		return
	}
	h.fullContentMu.Lock()
	if h.fullContentJobs == nil {
		h.fullContentJobs = map[int64]*fullContentJob{}
	}
	if job := h.fullContentJobs[item.ID]; job != nil && job.State == "running" {
		h.fullContentMu.Unlock()
		dataResponse(c, fullContentResponse{State: "running"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	job := &fullContentJob{State: "running", Cancel: cancel}
	h.fullContentJobs[item.ID] = job
	h.fullContentMu.Unlock()
	go func() {
		defer cancel()
		// A short queue prevents opening many Chrome instances when navigating quickly.
		h.fullContentWorker.Lock()
		defer h.fullContentWorker.Unlock()
		var content string
		var fetchErr error
		if ctx.Err() != nil {
			fetchErr = ctx.Err()
		} else if h.fetchFullContent != nil {
			content, fetchErr = h.fetchFullContent(ctx, item.Link)
		} else {
			content, fetchErr = fulltext.Fetch(ctx, h.config.FullContentNodePath, h.config.FullContentScriptPath, item.Link)
		}
		if fetchErr == nil && len(content) < 100 {
			fetchErr = errors.New("article body is empty")
		}
		if fetchErr == nil {
			fetchErr = h.store.SaveFullContent(item.ID, item.Link, content)
		}
		h.fullContentMu.Lock()
		defer h.fullContentMu.Unlock()
		job.State = "idle"
		job.Cancel = nil
		if fetchErr != nil {
			job.State = "error"
			job.Error = "Could not fetch the article. Open the original page or retry."
			slog.Warn("full content fetch failed", "item_id", item.ID, "error", fetchErr)
		}
	}()
	c.JSON(http.StatusAccepted, gin.H{"data": fullContentResponse{State: "running"}})
}

// CloseFullContent cancels browser jobs during server shutdown.
func (h *Handler) CloseFullContent() {
	h.fullContentMu.Lock()
	defer h.fullContentMu.Unlock()
	for _, job := range h.fullContentJobs {
		if job.Cancel != nil {
			job.Cancel()
		}
	}
}
