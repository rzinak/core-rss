package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"

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
	Items []Item `xml:"channel>item"`
}

type FeedFolder struct {
	Name  string
	Feeds []*Feed
}

func main() {
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
	statusBar.SetText("?: help | q: quit | Tab: switch focus | h/j/k/l: navigate")
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

	// here i add the addFeed page with visible set to false
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

	// grid := tview.NewGrid().
	// 	SetRows(0).
	// 	SetColumns(0).
	// 	AddItem(appFlex, 0, 0, 1, 1, 0, 0, true)

	// add a default folder
	defaultFolder := &FeedFolder{Name: "Default"}
	defaultFolderNode := tview.NewTreeNode(defaultFolder.Name).SetReference(defaultFolder)
	root.AddChild(defaultFolderNode)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		switch v := reference.(type) {
		case Item:
			// handle item selection
			plainText, err := html2text.FromString(v.Content, html2text.Options{PrettyTables: true})
			if err != nil {
				plainText = fmt.Sprintf("error converting html to text: %v: ", err)
			}
			contentView.Clear()
			fmt.Fprintf(contentView, "[yellow]Published:[~] %s\n\n%s", v.PubDate, plainText)
			contentView.SetTitle(v.Title)
			app.SetFocus(contentView)
		case *Feed:
			// handle feed selection (toggle feed items)
			if len(node.GetChildren()) > 0 {
				node.SetChildren(nil) // collapse here
			} else {
				// expand
				for _, item := range v.Items {
					item := item
					node.AddChild(tview.NewTreeNode(item.Title).SetReference(item))
				}
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
			if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
				log("error parsing feed: %v", err)
				app.QueueUpdateDraw(func() {
					statusBar.SetText(fmt.Sprintf("error parsing feed: %v", err))
				})
				return
			}

			log("feed title :%s", feed.Title)
			log("number of items: %d", len(feed.Items))

			// update ui on the main thread
			app.QueueUpdateDraw(func() {
				feedNode := tview.NewTreeNode(feed.Title).SetReference(&feed)
				root.AddChild(feedNode)

				//add feed items under the folder node
				for _, item := range feed.Items {
					item := item
					feedNode.AddChild(tview.NewTreeNode(item.Title).SetReference(item))
				}

				// clear input after adding the feed
				addFeedForm.GetFormItem(0).(*tview.InputField).SetText("")

				// update status bar
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

	tree.SetChangedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if feed, ok := reference.(*Feed); ok {
			// display the feed title in the content view
			contentView.Clear()
			fmt.Fprintf(contentView, "[yellow]Feed Title:[~] %s\n\n", feed.Title)
			contentView.SetTitle(feed.Title)
			app.SetFocus(contentView)
		}
	})

	// flex container to center the form
	formFlex.AddItem(nil, 0, 1, false)
	formFlex.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(addFeedForm, 0, 3, true).
		AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	// add form to a page
	// pages.AddPage("addFeed", formFlex, true, true)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

	// list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
	// 	switch event.Rune() {
	// 	case 'j':
	// 		currentIndex := list.GetCurrentItem()
	// 		if currentIndex < list.GetItemCount()-1 {
	// 			list.SetCurrentItem(currentIndex + 1)
	// 		}
	// 		return nil
	// 	case 'k':
	// 		currentIndex := list.GetCurrentItem()
	// 		if currentIndex > 0 {
	// 			list.SetCurrentItem(currentIndex - 1)
	// 		}
	// 		return nil
	// 	}
	// 	return event
	// })

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
