package main

import (
	"github.com/rzinak/core-rss/internal/models"
	"github.com/rzinak/core-rss/internal/services"
	"github.com/rzinak/core-rss/internal/ui"
	// "github.com/rzinak/core-rss/pkg/utils"
)

func main() {
	folderData, err := services.LoadFolders()

	if err != nil {
		folderData = &models.FolderData{
			Folders: []models.FeedFolder{{
				Name:  "Default",
				Feeds: []*models.Feed{},
			}},
		}
	}

	ui.SetupUI(folderData)
}
