package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HTMLEntityEntry represents an entity for display in the picker.
type HTMLEntityEntry struct {
	Entity      string
	Character   string
	Description string
}

// HTMLEntityDialog creates a searchable HTML entity picker dialog.
// onSelect is called with the entity string when the user picks one.
// onClose is called when the dialog is dismissed.
func HTMLEntityDialog(app *tview.Application, entities []HTMLEntityEntry, onSelect func(entity string), onClose func()) tview.Primitive {
	// Main layout
	layout := tview.NewFlex().SetDirection(tview.FlexRow)
	layout.SetBackgroundColor(ColorDialogBg)
	layout.SetBorder(true)
	layout.SetBorderColor(ColorStatusText)
	setModernTitle(layout, "Insert HTML Entity")
	layout.SetTitleColor(ColorStatusText)

	// Search input
	searchInput := tview.NewInputField()
	searchInput.SetLabel("Search: ")
	searchInput.SetBackgroundColor(ColorDialogBg)
	searchInput.SetFieldBackgroundColor(ColorBg)
	searchInput.SetFieldTextColor(ColorTextWhite)
	searchInput.SetLabelColor(ColorStatusText)

	// Entity list
	entityList := tview.NewList()
	entityList.SetBackgroundColor(ColorBg)
	entityList.SetMainTextColor(ColorTextWhite)
	entityList.SetSecondaryTextColor(ColorTextGray)
	entityList.SetSelectedTextColor(ColorSelectedText)
	entityList.SetSelectedBackgroundColor(ColorSelected)
	entityList.ShowSecondaryText(true)

	// Track filtered indices for selection callback
	var filteredIndices []int

	populateList := func(filter string) {
		entityList.Clear()
		filteredIndices = nil
		filter = strings.ToLower(filter)
		for i, ent := range entities {
			if filter != "" {
				matchTarget := strings.ToLower(ent.Entity + " " + ent.Description + " " + ent.Character)
				if !strings.Contains(matchTarget, filter) {
					continue
				}
			}
			idx := i
			filteredIndices = append(filteredIndices, idx)
			// Use tview.Escape to avoid bracket interpretation
			mainText := tview.Escape(ent.Entity + "  " + ent.Character)
			secondText := "  " + tview.Escape(ent.Description)
			entityList.AddItem(mainText, secondText, 0, func() {
				onSelect(entities[idx].Entity)
			})
		}
	}

	populateList("")

	// Wire search to filter
	searchInput.SetChangedFunc(func(text string) {
		populateList(text)
	})

	// Handle keyboard navigation
	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			onClose()
			return nil
		case tcell.KeyDown, tcell.KeyTab:
			app.SetFocus(entityList)
			return nil
		case tcell.KeyEnter:
			// Select the first item if list has entries
			if entityList.GetItemCount() > 0 {
				entityList.SetCurrentItem(0)
				if len(filteredIndices) > 0 {
					onSelect(entities[filteredIndices[0]].Entity)
				}
			}
			return nil
		}
		return event
	})

	entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			onClose()
			return nil
		case tcell.KeyTab:
			app.SetFocus(searchInput)
			return nil
		}
		// Allow typing to go back to search
		if event.Key() == tcell.KeyRune {
			app.SetFocus(searchInput)
			// Re-inject the character into the search input
			searchInput.SetText(searchInput.GetText() + string(event.Rune()))
			populateList(searchInput.GetText())
			return nil
		}
		return event
	})

	layout.AddItem(searchInput, 1, 0, true)
	layout.AddItem(entityList, 0, 1, false)

	return wrapDialogWithShadow(layout, 50, 20)
}
