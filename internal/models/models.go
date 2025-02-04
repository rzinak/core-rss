package models

import (
	"github.com/rivo/tview"
)

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"` // gotta map <content:encoded>
	// Content string `xml:"content:encoded" xml_namespace:"http://purl.org/rss/1.0/modules/content/"`
}

type Feed struct {
	Title string `xml:"channel>title" json:"title"`
	URL   string `json:"url"`
	Items []Item `xml:"channel>item" json:"-"`
}

type FeedFolder struct {
	Name       string          `json:"name"`
	Feeds      []*Feed         `json:"feeds"`
	FolderNode *tview.TreeNode `json:"-"`
}

type FeedData struct {
	Feeds []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"feeds"`
}

type FolderData struct {
	Folders []FeedFolder `json:"folders"`
}
