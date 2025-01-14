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
	// "github.com/rzinak/core-rss/pkg/utils"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
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

func SetupUI(folderData *models.FolderData) *tview.Pages {
	app := tview.NewApplication()

	if len(folderData.Folders) == 0 {
		folderData.Folders = append(folderData.Folders, models.FeedFolder{
			Name:  "Default",
			Feeds: []*models.Feed{},
		})
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

	defaultStatusBarMsg := "?: help | q: quit | Tab: switch focus | j/k: navigate | a: add new feed | d: remove a feed | f: add a folder | r: rename a folder | To see more, press '?'"

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
	helpModal.SetText(`Press 'q' to quit
		Press 'Tab' to switch focus inside the application
		Navigate using j/k
		Press 'Enter' to close this help (when focused)
		Press 'a' to add a new feed
		Press 'f' to add a folder
		Press 'r' to rename a folder
		Press 'Ctrl + O' to open the current post in the browser`)
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
	confirmModal.SetTitle("Remove feed")
	confirmModal.SetTitleColor(tcell.ColorGreen)
	confirmModal.SetButtonTextColor(tcell.ColorGreen)
	confirmModal.SetButtonBackgroundColor(tcell.ColorBlack)

	for i := range folderData.Folders {
		folder := &folderData.Folders[i]
		folderNode := tview.NewTreeNode(folder.Name).SetReference(folder)
		folderNode.SetColor(tcell.ColorGreen)
		folderNode.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color(tcell.ColorValues[0x000000])))
		folderNode.SetSelectedTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))

		folder.FolderNode = folderNode
		root.AddChild(folderNode)

		for _, feed := range folder.Feeds {
			feedNode := tview.NewTreeNode(feed.Title).SetReference(feed)
			feedNode.SetColor(tcell.ColorGreen)
			folderNode.AddChild(feedNode)
		}
	}

	resetStatusBarMsg := func(secondsToDisappear int) {
		go func() {
			time.Sleep(time.Duration(secondsToDisappear) * time.Second)
			app.QueueUpdateDraw(func() {
				statusBar.SetText(defaultStatusBarMsg)
			})
		}()
	}

	var currentItem *models.Item

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		switch v := reference.(type) {
		case models.Item:
			currentItem = &v
			var content string
			var err error
			if v.Content != "" {
				content, err = html2text.FromString(v.Content, html2text.Options{PrettyTables: true})
			} else {
				content, err = html2text.FromString(v.Description, html2text.Options{PrettyTables: true})
			}

			if err != nil {
				content = "error parsing content: " + err.Error()
				statusBar.SetText(content)
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

					decoder := xml.NewDecoder(resp.Body)

					decoder.CharsetReader = func(charsetLabel string, input io.Reader) (io.Reader, error) {
						logToFile(fmt.Sprintf("charSetLabel: %s", charsetLabel))
						return charset.NewReaderLabel(charsetLabel, input)
					}

					var feedData models.Feed
					// logToFile(fmt.Sprintf("feedData: %s", feedData))

					err = decoder.Decode(&feedData)
					if err != nil {
						return
					}

					// logToFile(fmt.Sprintf("&feedData: %s", &feedData))

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

					resetStatusBarMsg(5)
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
			selectedNode := tree.GetCurrentNode()
			var targetFolder *models.FeedFolder

			if selectedNode != nil {
				ref := selectedNode.GetReference()
				switch v := ref.(type) {
				case *models.FeedFolder:
					targetFolder = v
				case *models.Feed:
					// if a feed is selected, gotta search through folderData to find its parent folder
					for i := range folderData.Folders {
						folder := &folderData.Folders[i]
						for _, feed := range folder.Feeds {
							if feed == v {
								targetFolder = folder
								break
							}
						}
						if targetFolder != nil {
							break
						}
					}
				}
			}

			// if no folder is found, gotta use the first folder
			if targetFolder == nil && len(folderData.Folders) > 0 {
				targetFolder = &folderData.Folders[0]
			}

			if targetFolder != nil {
				feed, message, err := services.AddFeedToFolder(targetFolder, url)
				if err != nil {
					statusBar.SetText("Error: " + message)
					resetStatusBarMsg(5)
				} else {
					// here i add the feed node to the UI
					if targetFolder.FolderNode != nil {
						feedNode := tview.NewTreeNode(feed.Title).SetReference(feed)
						feedNode.SetColor(tcell.ColorGreen)
						targetFolder.FolderNode.AddChild(feedNode)
					}
					services.SaveFolders(folderData)
					statusBar.SetText(message)
					resetStatusBarMsg(5)
				}
			} else {
				statusBar.SetText("No folder available to add feed")
				resetStatusBarMsg(5)
			}
		}
		pages.HidePage("addFeed")
	})

	addFeedForm.SetButtonsAlign(1)

	addFeedForm.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFeedForm.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
	addFeedForm.SetTitleColor(tcell.ColorGreen)
	addFeedForm.SetFieldBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFeedForm.SetFieldTextColor(tcell.ColorGreen)
	addFeedForm.SetButtonTextColor(tcell.ColorGreen)
	addFeedForm.SetButtonBackgroundColor(tcell.ColorBlack)

	closingTipText := tview.NewTextView().
		SetText("Tip: Press 'ESC' to close this window")

	closingTipText.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	closingTipText.SetTextAlign(1)

	addFeedFormLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(addFeedForm, 0, 1, true).
		AddItem(closingTipText, 1, 0, false)

	addFeedFormLayout.SetBorder(true).
		SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen)).
		SetTitle("Add a new feed").
		SetTitleColor(tcell.ColorGreen)

	// flex container to center the form
	formFlex.AddItem(nil, 0, 1, false)
	formFlex.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(addFeedFormLayout, 70, 1, true).
		AddItem(nil, 0, 1, false), 8, 1, true).
		AddItem(nil, 0, 1, false)

	addFeedForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.HidePage("addFeed")
			app.SetFocus(tree)
			return nil
		}
		return event
	})

	addFolderForm := tview.NewForm()
	addFolderForm.SetFieldBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFolderForm.SetFieldTextColor(tcell.ColorGreen)
	addFolderForm.AddInputField("Folder Name: ", "", 0, nil, nil)
	addFolderForm.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
	addFolderForm.AddButton("Add", func() {
		folderName := addFolderForm.GetFormItem(0).(*tview.InputField).GetText()
		if folderName != "" {
			for _, folder := range folderData.Folders {
				if folder.Name == folderName {
					statusBar.SetText("Folder already exists!")
					resetStatusBarMsg(5)
					return
				}
			}

			newFolder := models.FeedFolder{
				Name:  folderName,
				Feeds: []*models.Feed{},
			}

			folderData.Folders = append(folderData.Folders, newFolder)

			newFolderNode := tview.NewTreeNode(folderName).SetReference(&newFolder)
			newFolderNode.SetColor(tcell.ColorGreen)
			newFolderNode.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.Color(tcell.ColorValues[0x000000])))
			newFolderNode.SetSelectedTextStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
			root.AddChild(newFolderNode)

			services.SaveFolders(folderData)
			statusBar.SetText(fmt.Sprintf("Folder '%s' created successfully!", folderName))
			resetStatusBarMsg(5)
		}
		pages.HidePage("addFolder")
		app.SetFocus(tree)
	})
	addFolderForm.SetButtonsAlign(1)
	addFolderForm.SetButtonTextColor(tcell.ColorGreen)
	addFolderForm.SetButtonBackgroundColor(tcell.ColorBlack)

	folderFormTipText := tview.NewTextView()
	folderFormTipText.SetText("Tip: Press 'ESC' to close")
	folderFormTipText.SetTextAlign(1)
	folderFormTipText.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))

	folderFormLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(addFolderForm, 5, 1, true).
		AddItem(folderFormTipText, 1, 0, false)
	folderFormLayout.SetBorder(true).
		SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen)).
		SetTitle("Add a new folder").
		SetTitleColor(tcell.ColorGreen)

	folderFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(folderFormLayout, 70, 1, true).
				AddItem(nil, 0, 1, false),
				8, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)

	pages.AddPage("addFolder", folderFlex, true, false)

	addFolderForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.HidePage("addFolder")
			app.SetFocus(tree)
			return nil
		}
		return event
	})

	addFeedForm.GetButton(0).SetSelectedFunc(func() {
		url := addFeedForm.GetFormItem(0).(*tview.InputField).GetText()
		if url == "" {
			return
		}

		selectedNode := tree.GetCurrentNode()
		var targetFolder *models.FeedFolder

		// here i determine target folder based on selection
		if selectedNode != nil {
			ref := selectedNode.GetReference()
			switch v := ref.(type) {
			case *models.FeedFolder:
				targetFolder = v
			case *models.Feed:
				// if a feed is selected, will use the root's first child (folder)
				if len(root.GetChildren()) > 0 {
					if folder, ok := root.GetChildren()[0].GetReference().(*models.FeedFolder); ok {
						targetFolder = folder
					}
				}
			}
		}

		// if no folder is selected/found, use the first available folder
		if targetFolder == nil && len(folderData.Folders) > 0 {
			targetFolder = &folderData.Folders[0]
		}

		if targetFolder == nil {
			statusBar.SetText("No folder available to add feed")
			resetStatusBarMsg(5)
			return
		}

		// here we feed to selected folder
		feed, message, err := services.AddFeedToFolder(targetFolder, url)
		if err != nil {
			statusBar.SetText("Error: " + message)
			resetStatusBarMsg(5)
		} else {
			// find the folder node and add the feed
			for _, node := range root.GetChildren() {
				if folder, ok := node.GetReference().(*models.FeedFolder); ok {
					if folder == targetFolder {
						feedNode := tview.NewTreeNode(feed.Title).SetReference(feed)
						feedNode.SetColor(tcell.ColorGreen)
						node.AddChild(feedNode)
						break
					}
				}
			}
			statusBar.SetText(message)
			resetStatusBarMsg(5)
		}
		pages.HidePage("addFeed")
		app.SetFocus(tree)
	})

	showRenameFolderModal := func(folder *models.FeedFolder, node *tview.TreeNode) {
		if pages.HasPage("renameFolder") {
			pages.RemovePage("renameFolder")
		}

		renameForm := tview.NewForm()
		renameForm.AddInputField("New Name: ", folder.Name, 0, nil, nil)
		renameForm.AddButton("Rename", func() {
			newName := renameForm.GetFormItem(0).(*tview.InputField).GetText()
			if newName == "" {
				statusBar.SetText("Folder name cannot be empty")
				resetStatusBarMsg(5)
				return
			}

			for _, f := range folderData.Folders {
				if f.Name == newName && &f != folder {
					statusBar.SetText("Folder name already exists")
					resetStatusBarMsg(5)
					return
				}
			}

			folder.Name = newName
			node.SetText(newName)
			services.SaveFolders(folderData)
			pages.HidePage("renameFolder")
			app.SetFocus(tree)
			statusBar.SetText(fmt.Sprintf("Folder renamed to '%s'", newName))
			resetStatusBarMsg(5)
		})

		renameForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEsc {
				pages.HidePage("renameFolder")
				app.SetFocus(tree)
				return nil
			}
			return event
		})

		tipText := tview.NewTextView()
		tipText.SetText("Tip: Press 'ESC' to close")
		tipText.SetTextAlign(1)
		tipText.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
		renameForm.SetButtonsAlign(1)
		renameForm.SetBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
		renameForm.SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
		renameForm.SetTitleColor(tcell.ColorGreen)
		renameForm.SetFieldBackgroundColor(tcell.Color(tcell.ColorValues[0x000000]))
		renameForm.SetFieldTextColor(tcell.ColorGreen)
		renameForm.SetButtonTextColor(tcell.ColorGreen)
		renameForm.SetButtonBackgroundColor(tcell.ColorBlack)

		renameFormLayout := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(renameForm, 5, 1, true).
			AddItem(tipText, 1, 0, false)

		renameFormLayout.SetBorder(true).
			SetBorderStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen)).
			SetTitle("Rename Folder").
			SetTitleColor(tcell.ColorGreen)

		renameFlex := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(tview.NewFlex().
					AddItem(nil, 0, 1, false).
					AddItem(renameFormLayout, 70, 1, true).
					AddItem(nil, 0, 1, false),
					8, 1, true).
				AddItem(nil, 0, 1, false),
				0, 1, true).
			AddItem(nil, 0, 1, false)

		pages.AddPage("renameFolder", renameFlex, true, false)
		pages.ShowPage("renameFolder")
		app.SetFocus(renameForm.GetFormItem(0).(*tview.InputField))
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, isInputField := app.GetFocus().(*tview.InputField); isInputField {
			return event
		}
		switch event.Rune() {
		case 'r':
			selectedNode := tree.GetCurrentNode()
			if selectedNode != nil {
				ref := selectedNode.GetReference()
				if folder, ok := ref.(*models.FeedFolder); ok {
					showRenameFolderModal(folder, selectedNode)
					return nil
				}
			}
			statusBar.SetText("No folder selected to rename")
			resetStatusBarMsg(5)
			return nil
		case 'f':
			pages.ShowPage("addFolder")
			addFolderForm.GetFormItem(0).(*tview.InputField).SetText("")
			app.SetFocus(addFolderForm)
			return nil
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
							//here i find the folder containing this feed
							var targetFolder *models.FeedFolder
							for i := range folderData.Folders {
								folder := &folderData.Folders[i]
								for j, f := range folder.Feeds {
									if f == feed {
										folder.Feeds = append(folder.Feeds[:j], folder.Feeds[j+1:]...)
										targetFolder = folder
										break
									}
								}
								if targetFolder != nil {
									break
								}
							}

							if targetFolder != nil && targetFolder.FolderNode != nil {
								targetFolder.FolderNode.RemoveChild(selectedNode)
								services.SaveFolders(folderData)
								statusBar.SetText(fmt.Sprintf("Feed '%s' removed.", feed.Title))
								contentView.Clear()
							}
						}
						pages.RemovePage("confirmRemove")
						app.SetFocus(tree)
						resetStatusBarMsg(5)
					})
					pages.AddPage("confirmRemove", confirmModal, true, true)
					app.SetFocus(confirmModal)
					return nil
				}
			}
			statusBar.SetText("No feed selected to remove. Please select a feed")
			resetStatusBarMsg(5)
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

	openURL := func(rawUrl string) error {
		parsedURL, err := url.Parse(rawUrl)
		if err != nil {
			return err
		}

		if parsedURL.Scheme == "" {
			parsedURL.Scheme = "http"
		}

		cmd := exec.Command("xdg-open", parsedURL.String())
		// cmd := exec.Command("open", parsedURL.String()) // for mac os i think i gotta use this
		// cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", parsedURL.String()) // and this one for windows

		return cmd.Start()

	}

	contentView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlO {
			if currentItem != nil && currentItem.Link != "" {
				err := openURL(currentItem.Link)
				if err != nil {
					statusBar.SetText("Error opening URL: " + err.Error())
					resetStatusBarMsg(5)
				} else {
					statusBar.SetText("Opening URL in browser...")
					resetStatusBarMsg(5)
				}
			} else {
				statusBar.SetText("No URL available to open")
				resetStatusBarMsg(5)
			}
			return nil
		}
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
