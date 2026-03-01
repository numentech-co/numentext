package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MenuItem represents a single menu item
type MenuItem struct {
	Label    string
	Shortcut string
	Action   func()
	Disabled bool
}

// Menu represents a dropdown menu
type Menu struct {
	Label  string
	Items  []*MenuItem
	OnOpen func() // Called before the dropdown opens, for dynamic menus
}

// MenuBar is a horizontal menu bar with dropdown support
type MenuBar struct {
	*tview.Box
	menus        []*Menu
	activeMenu   int // -1 = none active
	activeItem   int
	dropdownOpen bool
	onAction     func()
}

func NewMenuBar() *MenuBar {
	mb := &MenuBar{
		Box:        tview.NewBox(),
		activeMenu: -1,
		activeItem: 0,
	}
	mb.SetBackgroundColor(ColorMenuBg)
	return mb
}

func (mb *MenuBar) SetOnAction(fn func()) {
	mb.onAction = fn
}

func (mb *MenuBar) AddMenu(menu *Menu) {
	mb.menus = append(mb.menus, menu)
}

func (mb *MenuBar) IsOpen() bool {
	return mb.dropdownOpen
}

func (mb *MenuBar) Close() {
	mb.dropdownOpen = false
	mb.activeMenu = -1
	mb.activeItem = 0
}

func (mb *MenuBar) Open(menuIdx int) {
	if menuIdx >= 0 && menuIdx < len(mb.menus) {
		if mb.menus[menuIdx].OnOpen != nil {
			mb.menus[menuIdx].OnOpen()
		}
		mb.activeMenu = menuIdx
		mb.activeItem = 0
		mb.dropdownOpen = true
	}
}

func (mb *MenuBar) Draw(screen tcell.Screen) {
	mb.Box.DrawForSubclass(screen, mb)
	x, y, width, _ := mb.GetInnerRect()

	// Draw menu bar background
	for cx := x; cx < x+width; cx++ {
		screen.SetContent(cx, y, ' ', nil, tcell.StyleDefault.Background(ColorMenuBg))
	}

	// Draw menu labels
	cx := x + 1
	for i, menu := range mb.menus {
		label := " " + menu.Label + " "
		style := tcell.StyleDefault.Foreground(ColorStatusText).Background(ColorMenuBg)
		if i == mb.activeMenu {
			style = tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)
		}
		for _, ch := range label {
			if cx < x+width {
				screen.SetContent(cx, y, ch, nil, style)
				cx++
			}
		}
	}

	// Draw dropdown if open
	if mb.dropdownOpen && mb.activeMenu >= 0 && mb.activeMenu < len(mb.menus) {
		mb.drawDropdown(screen, x, y)
	}
}

func (mb *MenuBar) drawDropdown(screen tcell.Screen, startX, startY int) {
	menu := mb.menus[mb.activeMenu]

	// Calculate dropdown position
	dropX := startX + 1
	for i := 0; i < mb.activeMenu; i++ {
		dropX += len(mb.menus[i].Label) + 2
	}
	dropY := startY + 1

	// Calculate dropdown width
	maxWidth := 0
	for _, item := range menu.Items {
		w := len(item.Label)
		if item.Shortcut != "" {
			w += 2 + len(item.Shortcut)
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += 4 // padding

	// Draw dropdown items
	for i, item := range menu.Items {
		iy := dropY + i

		style := tcell.StyleDefault.Foreground(ColorMenuDropText).Background(ColorMenuDropBg)
		shortcutStyle := tcell.StyleDefault.Foreground(ColorMenuShortcut).Background(ColorMenuDropBg)
		if i == mb.activeItem {
			style = tcell.StyleDefault.Foreground(ColorMenuHlText).Background(ColorMenuHighlight)
			shortcutStyle = tcell.StyleDefault.Foreground(ColorTextGray).Background(ColorMenuHighlight)
		}
		if item.Disabled {
			style = tcell.StyleDefault.Foreground(ColorMenuShortcut).Background(ColorMenuDropBg)
		}

		// Clear line
		for cx := dropX; cx < dropX+maxWidth; cx++ {
			screen.SetContent(cx, iy, ' ', nil, style)
		}

		// Draw label
		for j, ch := range item.Label {
			if dropX+2+j < dropX+maxWidth {
				screen.SetContent(dropX+2+j, iy, ch, nil, style)
			}
		}

		// Draw shortcut
		if item.Shortcut != "" {
			scX := dropX + maxWidth - len(item.Shortcut) - 1
			for j, ch := range item.Shortcut {
				screen.SetContent(scX+j, iy, ch, nil, shortcutStyle)
			}
		}
	}
}

func (mb *MenuBar) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return mb.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if !mb.dropdownOpen {
			return
		}

		menu := mb.menus[mb.activeMenu]

		switch event.Key() {
		case tcell.KeyLeft:
			mb.activeMenu--
			if mb.activeMenu < 0 {
				mb.activeMenu = len(mb.menus) - 1
			}
			if mb.menus[mb.activeMenu].OnOpen != nil {
				mb.menus[mb.activeMenu].OnOpen()
			}
			mb.activeItem = 0
		case tcell.KeyRight:
			mb.activeMenu++
			if mb.activeMenu >= len(mb.menus) {
				mb.activeMenu = 0
			}
			if mb.menus[mb.activeMenu].OnOpen != nil {
				mb.menus[mb.activeMenu].OnOpen()
			}
			mb.activeItem = 0
		case tcell.KeyUp:
			mb.activeItem--
			if mb.activeItem < 0 {
				mb.activeItem = len(menu.Items) - 1
			}
		case tcell.KeyDown:
			mb.activeItem++
			if mb.activeItem >= len(menu.Items) {
				mb.activeItem = 0
			}
		case tcell.KeyEnter:
			if mb.activeItem >= 0 && mb.activeItem < len(menu.Items) {
				item := menu.Items[mb.activeItem]
				if item.Action != nil && !item.Disabled {
					mb.Close()
					if mb.onAction != nil {
						mb.onAction()
					}
					item.Action()
				}
			}
		case tcell.KeyEscape:
			mb.Close()
			if mb.onAction != nil {
				mb.onAction()
			}
		}
	})
}

func (mb *MenuBar) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return mb.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		mx, my := event.Position()
		bx, by, _, _ := mb.GetInnerRect()

		if action != tview.MouseLeftClick {
			return false, nil
		}

		// Click on menu bar
		if my == by {
			setFocus(mb)
			cx := bx + 1
			for i, menu := range mb.menus {
				labelLen := len(menu.Label) + 2
				if mx >= cx && mx < cx+labelLen {
					if mb.dropdownOpen && mb.activeMenu == i {
						mb.Close()
						if mb.onAction != nil {
							mb.onAction()
						}
					} else {
						mb.Open(i)
					}
					return true, mb
				}
				cx += labelLen
			}
			return false, nil
		}

		// Click in dropdown
		if mb.dropdownOpen && mb.activeMenu >= 0 {
			menu := mb.menus[mb.activeMenu]
			dropY := by + 1
			itemIdx := my - dropY
			if itemIdx >= 0 && itemIdx < len(menu.Items) {
				item := menu.Items[itemIdx]
				if item.Action != nil && !item.Disabled {
					mb.Close()
					if mb.onAction != nil {
						mb.onAction()
					}
					item.Action()
				}
				return true, nil
			}
		}

		// Click elsewhere closes menu
		if mb.dropdownOpen {
			mb.Close()
			if mb.onAction != nil {
				mb.onAction()
			}
			return false, nil
		}

		return false, nil
	})
}

func (mb *MenuBar) GetDropdownRect() (int, int, int, int) {
	if !mb.dropdownOpen || mb.activeMenu < 0 {
		return 0, 0, 0, 0
	}

	bx, by, _, _ := mb.GetInnerRect()
	menu := mb.menus[mb.activeMenu]

	dropX := bx + 1
	for i := 0; i < mb.activeMenu; i++ {
		dropX += len(mb.menus[i].Label) + 2
	}
	dropY := by + 1

	maxWidth := 0
	for _, item := range menu.Items {
		w := len(item.Label)
		if item.Shortcut != "" {
			w += 2 + len(item.Shortcut)
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += 4

	return dropX, dropY, maxWidth, len(menu.Items)
}
