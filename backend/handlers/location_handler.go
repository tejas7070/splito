package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"tripledger/services"
)

type LocationHandler struct {
	placeService *services.PlaceService
}

func NewLocationHandler(placeService *services.PlaceService) *LocationHandler {
	return &LocationHandler{placeService: placeService}
}

func (h *LocationHandler) Search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) < 3 || len([]rune(query)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search must be between 3 and 100 characters"})
		return
	}

	result, err := h.placeService.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
