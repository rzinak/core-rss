package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jaytaylor/html2text"
	"github.com/rivo/tview"
)

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"` // gotta map <content:encoded>
}

type Feed struct {
	Title string `xml:"channel>title"`
	URL   string
	Items []Item `xml:"channel>item"`
}

type FeedFolder struct {
	Name  string
	Feeds []*Feed
}

type FeedData struct {
	Feeds []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"feeds"`
}

func loadFeeds(folder *FeedFolder, root *tview.TreeNode) {
	var data FeedData
	file, err := os.Open("feeds.json")
	if err != nil {
		if os.IsNotExist(err) {
			folder.Feeds = []*Feed{}
			return
		}
		fmt.Println("error loading feeds: ", err)
		return
	}

	defer file.Close()
	err = json.NewDecoder(file).Decode(&data)
	if err != nil {
		fmt.Println("error parsing feeds: ", err)
		return
	}

	folder.Feeds = make([]*Feed, len(data.Feeds))
	for i, f := range data.Feeds {
		feed := Feed{
			Title: f.Title,
			URL:   f.URL,
		}

		folder.Feeds[i] = &feed

		feedNode := tview.NewTreeNode(feed.Title).SetReference(&feed)
		feedNode.SetColor(tcell.ColorGreen)
		folderNode := findNode(root, func(node *tview.TreeNode) bool {
			return node.GetReference() == folder
		})
		if folderNode != nil {
			folderNode.AddChild(feedNode)
		}
	}
}

func saveFeeds(folder *FeedFolder) {
	var data FeedData
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
		fmt.Println("error saving feeds: ", err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	err = encoder.Encode(&data)
	if err != nil {
		fmt.Println("error encoding feeds: ", err)
	}
}

func findNode(root *tview.TreeNode, condition func(*tview.TreeNode) bool) *tview.TreeNode {
	if condition(root) {
		return root
	}
	for _, child := range root.GetChildren() {
		if found := findNode(child, condition); found != nil {
			return found
		}
	}
	return nil
}

func main() {
	defaultStatusBarMsg := "?: help | q: quit | Tab: switch focus | h/j/k/l: navigate"

	logFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	log := func(format string, args ...interface{}) {
		fmt.Fprintf(logFile, format+"\n", args...)
		logFile.Sync()
	}

	log("starting...")

	// create tui application
	app := tview.NewApplication()

	// create a tree view for folders and feeds
	tree := tview.NewTreeView()
	root := tview.NewTreeNode("Feeds").SetColor(tcell.ColorGreen)
	tree.SetRoot(root).SetCurrentNode(root)
	tree.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	tree.SetBorder(true)
	tree.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	tree.SetTitleColor(tcell.ColorGreen)
	tree.SetTitle("Core RSS")

	// create a textview to display the content of the selected item
	contentView := tview.NewTextView()
	contentView.SetDynamicColors(true)
	contentView.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	contentView.SetScrollable(true)
	contentView.SetWrap(true)
	contentView.SetBorder(true)
	contentView.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	contentView.SetTitleColor(tcell.Color(tcell.ColorValues[0xFFFFFF]))

	// status bar
	statusBar := tview.NewTextView()
	statusBar.SetTextAlign(tview.AlignLeft)
	statusBar.SetText(defaultStatusBarMsg)
	statusBar.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x333333]))
	statusBar.SetTextColor(tcell.Color(tcell.ColorValues[0xFFFFFF]))

	mainFlex := tview.NewFlex().
		AddItem(tree, 0, 1, true).
		AddItem(contentView, 0, 2, true)

	formFlex := tview.NewFlex()
	formFlex.SetDirection(tview.FlexRow)

	appFlex := tview.NewFlex()
	appFlex.SetDirection(tview.FlexRow)

	pages := tview.NewPages().AddPage("main", appFlex, true, true)
	pages.AddPage("addFeed", formFlex, true, false) // initially hidden

	appFlex.AddItem(mainFlex, 0, 1, true)
	appFlex.AddItem(statusBar, 1, 1, false)

	helpModal := tview.NewModal()
	helpModal.SetText("Press 'q' to quit\nPress '?' to close this help")
	helpModal.AddButtons([]string{"Close"})
	helpModal.SetBorder(true)
	helpModal.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	helpModal.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	helpModal.SetTextColor(tcell.Color(tcell.ColorValues[0xFFFFFF]))
	helpModal.SetTitle("Help")
	helpModal.SetTitleColor(tcell.ColorGreen)

	// add a default folder
	defaultFolder := &FeedFolder{Name: "Default"}
	defaultFolderNode := tview.NewTreeNode(defaultFolder.Name).SetReference(defaultFolder)
	defaultFolderNode.SetColor(tcell.ColorGreen)
	root.AddChild(defaultFolderNode)

	loadFeeds(defaultFolder, root)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		switch v := reference.(type) {
		case Item:
			// handle item selection
			plainText, err := html2text.FromString(v.Content, html2text.Options{PrettyTables: true})
			if err != nil {
				plainText = fmt.Sprintf("error converting html to text: %v: ", err)
			}
			log("plainText: %s", plainText)
			log("v.Content: %s", v.Content)
			contentView.Clear()
			fmt.Fprintf(contentView, "[yellow]Published:[~] %s\n\n%s", v.PubDate, plainText)
			contentView.SetTitle(v.Title)
			app.SetFocus(contentView)
		case *Feed:
			if len(node.GetChildren()) > 0 {
				node.SetChildren(nil)
			} else {
				go func() {
					resp, err := http.Get(v.URL)
					if err != nil {
						log("error fetching feed items: %v", err)
						return
					}
					defer resp.Body.Close()

					body, err := io.ReadAll(resp.Body)
					resp.Body = io.NopCloser(bytes.NewReader(body))

					var feedData Feed
					err = xml.NewDecoder(resp.Body).Decode(&feedData)
					if err != nil {
						log("error parsing feed items: %v", err)
						return
					}

					app.QueueUpdateDraw(func() {
						for _, item := range feedData.Items {
							itemCopy := item
							feedItemNode := tview.NewTreeNode(itemCopy.Title).SetReference(itemCopy)
							feedItemNode.SetColor(tcell.ColorGreen)
							node.AddChild(feedItemNode)
						}
						statusBar.SetText(fmt.Sprintf("loaded %d items for feed: %s", len(feedData.Items), v.Title))
					})

					go func() {
						time.Sleep(5 * time.Second)
						app.QueueUpdateDraw(func() {
							statusBar.SetText(defaultStatusBarMsg)
						})
					}()
				}()
			}
		case *FeedFolder:
			// handle folder selection (toggle feeds)
			if len(node.GetChildren()) > 0 {
				node.SetChildren(nil) // collapse here
			} else {
				// expand
				for _, feed := range v.Feeds {
					feedNode := tview.NewTreeNode(feed.Title).SetReference(feed)
					node.AddChild(feedNode)
				}
			}
		}
	})

	// modal to add a new feed
	addFeedForm := tview.NewForm()

	// function to add a feed to a folder
	addFeedToFolder := func(folder *FeedFolder, feedUrl string) {
		go func() {
			log("Fetching feed from URL: %s", feedUrl)

			resp, err := http.Get(feedUrl)
			if err != nil {
				log("Error fetching feed: %v", err)
				app.QueueUpdateDraw(func() {
					statusBar.SetText(fmt.Sprintf("error fetching feed: %v", err))
				})
				return
			}
			defer resp.Body.Close()
			var feed Feed
			feed.URL = feedUrl
			err = xml.NewDecoder(resp.Body).Decode(&feed)

			if err != nil {
				log("error parsing feed: %v", err)
				app.QueueUpdateDraw(func() {
					statusBar.SetText(fmt.Sprintf("error parsing feed: %v", err))
				})
				return
			}

			log("feed title :%s", feed.Title)
			log("number of items: %d", len(feed.Items))

			app.QueueUpdateDraw(func() {
				folder.Feeds = append(folder.Feeds, &feed)
				feedNode := tview.NewTreeNode(feed.Title).SetReference(&feed)
				feedNode.SetColor(tcell.ColorGreen)
				folderNode := findNode(root, func(node *tview.TreeNode) bool {
					return node.GetReference() == folder
				})
				if folderNode != nil {
					folderNode.AddChild(feedNode)
				}
				saveFeeds(folder)
				statusBar.SetText(fmt.Sprintf("added feed: %s (%d items)", feed.Title, len(feed.Items)))
			})
		}()
	}

	addFeedForm.AddInputField("RSS Feed URL: ", "", 0, nil, nil)
	addFeedForm.AddButton("Add", func() {
		url := addFeedForm.GetFormItem(0).(*tview.InputField).GetText()
		if url != "" {
			addFeedToFolder(defaultFolder, url)
		}
		pages.HidePage("addFeed")
	})
	addFeedForm.AddButton("Cancel", func() {
		pages.HidePage("addFeed")
	})
	addFeedForm.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFeedForm.SetBorder(true)
	addFeedForm.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	addFeedForm.SetTitleColor(tcell.ColorGreen)
	addFeedForm.SetTitle("Add a new feed")

	// flex container to center the form
	formFlex.AddItem(nil, 0, 1, false)
	formFlex.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(addFeedForm, 0, 3, true).
		AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, isInputField := app.GetFocus().(*tview.InputField); isInputField {
			return event
		}
		switch event.Rune() {
		case 'a':
			pages.ShowPage("addFeed")
			addFeedForm.GetFormItem(0).(*tview.InputField).SetText("")
			app.SetFocus(addFeedForm)
			return nil
		case 'q':
			app.Stop()
			return nil
		case '?':
			pages.AddPage("help", helpModal, true, true)
			app.SetFocus(helpModal)
			return nil
		case rune(tcell.KeyTab):
			if app.GetFocus() == tree {
				app.SetFocus(contentView)
			} else {
				app.SetFocus(tree)
			}
			return nil
		}
		return event
	})

	helpModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		pages.RemovePage("help")
		app.SetFocus(appFlex)
	})

	contentView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			row, _ := contentView.GetScrollOffset()
			contentView.ScrollTo(row+1, 0)
			return nil
		case 'k':
			row, _ := contentView.GetScrollOffset()
			if row > 0 {
				contentView.ScrollTo(row-1, 0)
			}
			return nil
		case 'h':
			_, col := contentView.GetScrollOffset()
			if col > 0 {
				contentView.ScrollTo(0, col-1)
			}
			return nil
		case 'l':
			_, col := contentView.GetScrollOffset()
			contentView.ScrollTo(0, col+1)
			return nil
		case 'g':
			if event.Modifiers() == tcell.ModNone {
				contentView.ScrollToBeginning()
				return nil
			}
		case 'G':
			contentView.ScrollToEnd()
			return nil
		}

		switch event.Key() {
		case tcell.KeyCtrlE:
			row, _ := contentView.GetScrollOffset()
			contentView.ScrollTo(row+1, 0)
			return nil
		case tcell.KeyCtrlY:
			row, _ := contentView.GetScrollOffset()
			if row > 0 {
				contentView.ScrollTo(row-1, 0)
			}
			return nil
		case tcell.KeyCtrlD:
			row, _ := contentView.GetScrollOffset()
			if row > 10 {
				contentView.ScrollTo(row-10, 0)
			} else {
				contentView.ScrollTo(0, 0)
			}
			return nil
		case tcell.KeyCtrlF:
			row, _ := contentView.GetScrollOffset()
			contentView.ScrollTo(row+20, 0)
			return nil
		case tcell.KeyCtrlB:
			row, _ := contentView.GetScrollOffset()
			if row > 20 {
				contentView.ScrollTo(row-20, 0)
			} else {
				contentView.ScrollTo(0, 0)
			}
			return nil
		}
		return event
	})

	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}
}
