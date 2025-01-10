package main

import (
	"github.com/rzinak/core-rss/internal/models"
	"github.com/rzinak/core-rss/internal/ui"
	"github.com/rzinak/core-rss/pkg/utils"
)

func main() {
	logger := utils.GetLogger()
	defer logger.Close()

	logger.Log("starting application...")

	folder := &models.FeedFolder{
		Name: "rzinwq",
	}

	err := ui.SetupUI(folder)

	if err != nil {
		logger.Log("ui error: %v", err)
	}
}
