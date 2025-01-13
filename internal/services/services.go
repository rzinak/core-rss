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

func LoadFolders() (*models.FolderData, error) {
	file, err := os.Open("feeds.json")
	if err != nil {
		if os.IsNotExist(err) {
			return &models.FolderData{
				Folders: []models.FeedFolder{{
					Name:  "Default",
					Feeds: []*models.Feed{},
				}},
			}, nil
		}
		return nil, err
	}
	defer file.Close()

	var data models.FolderData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		// if theres an error decoding the new format, fallback to the old format
		file.Seek(0, 0) // reset file pointer
		var oldData struct {
			Feeds []struct {
				Title string `jsokn:"title"`
				URL   string `json:"url"`
			} `json:"feeds"`
		}
		if err := json.NewDecoder(file).Decode(&oldData); err != nil {
			return nil, err
		}

		// convert old format to new format
		data.Folders = []models.FeedFolder{{
			Name:  "Default",
			Feeds: make([]*models.Feed, len(oldData.Feeds)),
		}}

		for i, f := range oldData.Feeds {
			data.Folders[0].Feeds[i] = &models.Feed{
				Title: f.Title,
				URL:   f.URL,
			}
		}
	}

	return &data, nil
}

func SaveFolders(data *models.FolderData) error {
	file, err := os.Create("feeds.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(data)
}

// now that i have the saveFolder func, this becomes deprecated
func SaveFeeds(folder *models.FeedFolder) error {
	data, err := LoadFolders()
	if err != nil {
		return err
	}

	found := false
	for i, f := range data.Folders {
		if f.Name == folder.Name {
			data.Folders[i].Feeds = folder.Feeds
			found = true
			break
		}
	}

	if !found {
		data.Folders = append(data.Folders, *folder)
	}

	return SaveFolders(data)
}

func AddFeedToFolder(folder *models.FeedFolder, feedUrl string) (*models.Feed, string, error) {
	for _, existingFeed := range folder.Feeds {
		if existingFeed.URL == feedUrl {
			return nil, "Feed already exists!", fmt.Errorf("feed with URL %s already exists", feedUrl)
		}
	}

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
	err = decoder.Decode(&feed)
	if err != nil {
		logToFile(fmt.Sprintf("error parsing: %v", err))
		return nil, "Failed to parse RSS feed", err
	}

	feed.URL = feedUrl
	folder.Feeds = append(folder.Feeds, &feed)

	data, err := LoadFolders()
	if err != nil {
		return nil, "Failed to load folders", err
	}

	for i := range data.Folders {
		if data.Folders[i].Name == folder.Name {
			data.Folders[i].Feeds = folder.Feeds
			break
		}
	}

	err = SaveFolders(data)
	if err != nil {
		return nil, "Failed to save feed", err
	}

	return &feed, fmt.Sprintf("Feed %s added successfully!", feed.Title), nil
}
