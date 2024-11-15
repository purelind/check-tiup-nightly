package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/purelind/check-tiup-nightly/internal/checker"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
	"github.com/purelind/check-tiup-nightly/internal/database"
)

type Handler struct {
	db *database.DB
}

func NewHandler(db *database.DB) *Handler {
	return &Handler{db: db}
}

// Error custom error type
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func NewError(status int, message string) *Error {
	return &Error{
		Status:  status,
		Message: message,
	}
}

func (h *Handler) ReportStatus(c *gin.Context) {
	var report checker.CheckReport
	if err := c.ShouldBindJSON(&report); err != nil {
		logger.Error("Invalid request body. Error:", err)
		logger.Error("Request body:", c.Request.Body)
		c.Error(NewError(http.StatusBadRequest, "Invalid request body: " + err.Error()))
		return
	}

	if err := h.db.SaveCheckResult(c.Request.Context(), &report); err != nil {
		logger.Error("Failed to save check result:", err)
		c.Error(NewError(http.StatusInternalServerError, "Failed to save check result"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func (h *Handler) GetLatestResults(c *gin.Context) {
	results, err := h.db.GetLatestResults(c.Request.Context())
	if err != nil {
		logger.Error("Failed to get latest results:", err)
		c.Error(NewError(http.StatusInternalServerError, "Failed to fetch latest results"))
		return
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetPlatformResults(c *gin.Context) {
	platform := c.Param("platform")
	if !ValidPlatforms[platform] {
		c.Error(NewError(http.StatusBadRequest, "Invalid platform"))
		return
	}

	params := database.QueryParams{
		Platform: platform,
		Days: 0,
	}

	// parse query parameters
	if days := c.Query("days"); days != "" {
		// query by days
		if val, err := strconv.Atoi(days); err == nil && val > 0 {
			params.Days = val
			params.QueryType = database.QueryByDays
		} else {
			c.Error(NewError(http.StatusBadRequest, "Invalid days parameter"))
			return
		}
	} else if limit := c.Query("limit"); limit != "" {
		// query by limit
		if val, err := strconv.Atoi(limit); err == nil && val > 0 {
			params.Limit = val
			params.QueryType = database.QueryByLimit
			params.Days = 0
		} else {
			c.Error(NewError(http.StatusBadRequest, "Invalid limit parameter"))
			return
		}
	} else {
		// default query the latest 10 records
		params.Limit = 10
		params.QueryType = database.QueryByLimit
		params.Days = 0
	}

	// add debug log
	logger.Info("Query params:", params)

	results, err := h.db.GetPlatformResults(c.Request.Context(), params)
	if err != nil {
		logger.Error("Failed to get platform results:", err)
		// output more detailed error information
		logger.Error("Detailed error:", err.Error())
		c.Error(NewError(http.StatusInternalServerError, "Failed to fetch platform results"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"platform": platform,
		"total":    len(results),
		"results":  results,
		"query_type": params.QueryType,
		"days":     params.Days,
		"limit":    params.Limit,
	})
}

func (h *Handler) GetPlatformHistory(c *gin.Context) {
	platform := c.Param("platform")
	if !ValidPlatforms[platform] {
		c.Error(NewError(http.StatusBadRequest, "Invalid platform"))
		return
	}

	days := 1 // default value
	if d := c.Query("days"); d != "" {
		if val, err := strconv.Atoi(d); err == nil && val > 0 {
			days = val
		}
	}

	params := database.QueryParams{
		Platform: platform,
		Days:     days,
	}

	results, err := h.db.GetPlatformHistory(c.Request.Context(), params)
	if err != nil {
		logger.Error("Failed to get platform history:", err)
		c.Error(NewError(http.StatusInternalServerError, "Failed to fetch platform history"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"platform": platform,
		"days":     days,
		"total":    len(results),
		"results":  results,
	})
}
