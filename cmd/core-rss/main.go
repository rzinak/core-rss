package main

import (
	"encoding/xml"
	"fmt"
	"net/http"

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
	Items []Item `xml:"channel>item"`
}

type FeedFolder struct {
	Name  string
	Feeds []*Feed
}

func main() {
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
	root.AddChild(tview.NewTreeNode(defaultFolder.Name).SetReference(defaultFolder))

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if item, ok := reference.(Item); ok {
			plainText, err := html2text.FromString(item.Content, html2text.Options{PrettyTables: true})
			if err != nil {
				plainText = fmt.Sprintf("error converting html to text: %v; ", err)
			}
			contentView.Clear()
			fmt.Fprintf(contentView, "[yellow]Published:[~] %s\n\n%s", item.PubDate, plainText)
			contentView.SetTitle(item.Title)
			app.SetFocus(contentView)
		}
	})

	// function to add a feed to a folder
	addFeedToFolder := func(folder *FeedFolder, feedUrl string) {
		resp, err := http.Get(feedUrl)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		var feed Feed
		if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
			return
		}

		folder.Feeds = append(folder.Feeds, &feed)
		folderNode := root.GetChildren()[0] // we do this assuming Default folder is first
		for _, item := range feed.Items {
			item := item
			folderNode.AddChild(tview.NewTreeNode(item.Title).SetReference(item))
		}
	}

	// modal to add a new feed
	addFeedForm := tview.NewForm()
	addFeedForm.AddInputField("RSS Feed URL: ", "", 0, nil, nil)
	addFeedForm.AddButton("Add", func() {
		url := addFeedForm.GetFormItem(0).(*tview.InputField).GetText()
		if url != "" {
			addFeedToFolder(defaultFolder, url)
		}
		pages.RemovePage("addFeed")
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
