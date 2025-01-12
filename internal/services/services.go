package services

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/rzinak/core-rss/internal/models"
	"github.com/rzinak/core-rss/pkg/utils"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"os"
)

func logToFile(message string) {
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, message)
	if err != nil {
		fmt.Println("Error writing to log file:", err)
	}
}

func LoadFeeds(folder *models.FeedFolder) error {
	logger := utils.GetLogger()
	defer logger.Close()

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

func AddFeedToFolder(folder *models.FeedFolder, feedUrl string) (*models.Feed, string, error) {
	resp, err := http.Get(feedUrl)
	if err != nil {
		return nil, "Failed to fetch RSS feed", err
	}
	defer resp.Body.Close()

	decoder := xml.NewDecoder(resp.Body)

	decoder.CharsetReader = func(charsetLabel string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(charsetLabel, input)
	}

	var feed models.Feed

	logToFile(fmt.Sprintf("AddFeedToFolder | feed title: %s", feed.Title))
	err = decoder.Decode(&feed)
	if err != nil {
		logToFile(fmt.Sprintf("error parsing:"))
		logToFile(fmt.Sprintf("%v", err))
		return nil, "Failed to parse RSS feed", err
	}
	feed.URL = feedUrl
	folder.Feeds = append(folder.Feeds, &feed)
	err = SaveFeeds(folder)
	if err != nil {
		return nil, "Failed to save feed", err
	}

	return &feed, fmt.Sprintf("Feed %s added successfully!", feed.Title), nil
}
