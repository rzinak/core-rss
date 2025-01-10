package ui

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/jaytaylor/html2text"
	"github.com/rivo/tview"
	"github.com/rzinak/core-rss/internal/models"
	"github.com/rzinak/core-rss/internal/services"
	"io"
	"net/http"
	"time"
)

func SetupUI(folder *models.FeedFolder) *tview.Pages {
	app := tview.NewApplication()

	err := services.LoadFeeds(folder)
	if err != nil {
	}

	root := tview.NewTreeNode("Feeds").SetColor(tcell.ColorGreen)
	root.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color(tcell.ColorValues[0x000000])))
	root.SetSelectedTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))

	tree := tview.NewTreeView()
	tree.SetRoot(root).SetCurrentNode(root)
	tree.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	tree.SetBorder(true)
	tree.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	tree.SetTitleColor(tcell.ColorGreen)
	tree.SetTitle("Core RSS")

	contentView := tview.NewTextView()
	contentView.SetDynamicColors(true)
	contentView.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	contentView.SetScrollable(true)
	contentView.SetWrap(true)
	contentView.SetBorder(true)
	contentView.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	contentView.SetTitleColor(tcell.ColorGreen)
	contentView.SetTitle("Core RSS")

	defaultStatusBarMsg := "?: help | q: quit | Tab: switch focus | j/k: navigate | a: add new feed"

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
	helpModal.SetText("Press 'q' to quit\nPress 'Tab' to switch focus inside the application\nNavigate using j/k\nPress 'Enter' to close this help (when focused)\nPress 'a' to add a new feed")
	helpModal.AddButtons([]string{"Close"})
	helpModal.SetBorder(true)
	helpModal.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	helpModal.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	helpModal.SetTextColor(tcell.Color(tcell.ColorValues[0xFFFFFF]))
	helpModal.SetTitle("Help")
	helpModal.SetTitleColor(tcell.ColorGreen)
	helpModal.SetButtonTextColor(tcell.ColorGreen)
	helpModal.SetButtonBackgroundColor(tcell.ColorBlack)

	confirmModal := tview.NewModal()
	confirmModal.SetText("Are you sure you want to remove the feed?")
	confirmModal.AddButtons([]string{"Yes", "No"})
	confirmModal.SetBorder(true)
	confirmModal.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	confirmModal.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	confirmModal.SetTextColor(tcell.Color(tcell.ColorValues[0xFFFFFF]))
	confirmModal.SetTitle("Confirm Removal")
	confirmModal.SetTitleColor(tcell.ColorGreen)
	confirmModal.SetButtonTextColor(tcell.ColorGreen)
	confirmModal.SetButtonBackgroundColor(tcell.ColorBlack)

	folderNode := tview.NewTreeNode(folder.Name).SetReference(folder)
	folderNode.SetColor(tcell.ColorGreen)
	folderNode.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color(tcell.ColorValues[0x000000])))
	folderNode.SetSelectedTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))

	folder.FolderNode = folderNode
	root.AddChild(folderNode)

	services.LoadFeeds(folder)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		switch v := reference.(type) {
		case models.Item:
			// handle item selection
			var content string
			var err error
			if v.Content != "" {
				content, err = html2text.FromString(v.Content, html2text.Options{PrettyTables: true})
			} else {
				content, err = html2text.FromString(v.Description, html2text.Options{PrettyTables: true})
			}

			if err != nil {
				content = "error parsing content: " + err.Error()
			}

			contentView.Clear()
			fmt.Fprintf(contentView, "[yellow]Published: %s\n\n%s", v.PubDate, content)
			contentView.SetTitle(v.Title)
			contentView.SetTitleColor(tcell.ColorYellow)
			app.SetFocus(contentView)
		case *models.Feed:
			if len(node.GetChildren()) > 0 {
				node.SetChildren(nil)
			} else {
				go func() {
					resp, err := http.Get(v.URL)
					if err != nil {
						return
					}
					defer resp.Body.Close()

					body, err := io.ReadAll(resp.Body)
					resp.Body = io.NopCloser(bytes.NewReader(body))

					var feedData models.Feed
					err = xml.NewDecoder(resp.Body).Decode(&feedData)
					if err != nil {
						return
					}

					app.QueueUpdateDraw(func() {
						for _, item := range feedData.Items {
							itemCopy := item
							feedItemNode := tview.NewTreeNode(itemCopy.Title).SetReference(itemCopy)
							feedItemNode.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color(tcell.ColorValues[0x000000])))
							feedItemNode.SetSelectedTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
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
		case *models.FeedFolder:
			// handle folder selection (toggle feeds)
			if len(node.GetChildren()) > 0 {
				node.SetChildren(nil) // collapse here
			} else {
				// expand
				for _, feed := range v.Feeds {
					feedNode := tview.NewTreeNode(feed.Title).SetReference(feed)
					feedNode.SetColor(tcell.ColorGreen)
					node.AddChild(feedNode)
				}
			}
		}
	})

	addFeedForm := tview.NewForm()

	addFeedForm.AddInputField("RSS Feed URL: ", "", 0, nil, nil)
	addFeedForm.AddButton("Add", func() {
		url := addFeedForm.GetFormItem(0).(*tview.InputField).GetText()
		if url != "" {
			services.AddFeedToFolder(folder, url)
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
	addFeedForm.SetFieldBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFeedForm.SetFieldTextColor(tcell.ColorGreen)
	addFeedForm.SetButtonTextColor(tcell.ColorGreen)
	addFeedForm.SetButtonBackgroundColor(tcell.ColorBlack)

	// flex container to center the form
	formFlex.AddItem(nil, 0, 1, false)
	formFlex.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(addFeedForm, 0, 3, true).
		AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false)

	addFeedForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.HidePage("addFeed")
			app.SetFocus(tree)
			return nil
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, isInputField := app.GetFocus().(*tview.InputField); isInputField {
			return event
		}
		switch event.Rune() {
		case 'a':
			pages.ShowPage("addFeed")
			addFeedForm.GetFormItem(0).(*tview.InputField).SetText("")
			app.SetFocus(addFeedForm.GetFormItem(0).(*tview.InputField))
			return nil
		case 'q':
			app.Stop()
			return nil
		case '?':
			pages.AddPage("help", helpModal, true, true)
			app.SetFocus(helpModal)
			return nil
		case 'd':
			selectedNode := tree.GetCurrentNode()
			if selectedNode != nil && selectedNode.GetReference() != nil {
				if feed, ok := selectedNode.GetReference().(*models.Feed); ok {
					confirmModal.SetText(fmt.Sprintf("Are you sure you want to remove the feed '%s'?", feed.Title))
					confirmModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						if buttonLabel == "Yes" {
							for i, f := range folder.Feeds {
								if f == feed {
									folder.Feeds = append(folder.Feeds[:i], folder.Feeds[i+1:]...)
									break
								}
							}
							folderNode.RemoveChild(selectedNode)
							services.SaveFeeds(folder)
							statusBar.SetText(fmt.Sprintf("Feed '%s' removed.", feed.Title))
							contentView.Clear()
						}
						pages.RemovePage("confirmRemove")
						app.SetFocus(tree)
					})
					pages.AddPage("confirmRemove", confirmModal, true, true)
					app.SetFocus(confirmModal)
					return nil
				}
			}
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

	return pages
}
