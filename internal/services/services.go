package services

import (
	"encoding/json"
	"encoding/xml"
	"github.com/rzinak/core-rss/internal/models"
	"net/http"
	"os"
)

func LoadFeeds(folder *models.FeedFolder) error {
	file, err := os.Open("feeds.json")
	if err != nil {
		return err
	}
	defer file.Close()

	var data struct {
		Feeds []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"feeds"`
	}
	err = json.NewDecoder(file).Decode(&data)
	if err != nil {
		return err
	}

	folder.Feeds = make([]*models.Feed, len(data.Feeds))
	for i, f := range data.Feeds {
		feed := models.Feed{
			Title: f.Title,
			URL:   f.URL,
		}
		folder.Feeds[i] = &feed
	}
	return nil
}

func SaveFeeds(folder *models.FeedFolder) error {
	var data struct {
		Feeds []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"feeds"`
	}
	data.Feeds = make([]struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}, len(folder.Feeds))
	for i, feed := range folder.Feeds {
		data.Feeds[i] = struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		}{
			Title: feed.Title,
			URL:   feed.URL,
		}
	}
	file, err := os.Create("feeds.json")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(&data)
}

func AddFeedToFolder(folder *models.FeedFolder, feedUrl string) (*models.Feed, error) {
	resp, err := http.Get(feedUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var feed models.Feed
	err = xml.NewDecoder(resp.Body).Decode(&feed)
	if err != nil {
		return nil, err
	}
	feed.URL = feedUrl

	folder.Feeds = append(folder.Feeds, &feed)
	err = SaveFeeds(folder)
	if err != nil {
		return nil, err
	}

	return &feed, nil
}
