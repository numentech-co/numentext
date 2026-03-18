package filetree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"numentext/internal/ui"
)

// FileTree is a file browser panel
type FileTree struct {
	*tview.TreeView
	rootPath   string
	onFileOpen func(path string)
}

func New(rootPath string) *FileTree {
	ft := &FileTree{
		TreeView: tview.NewTreeView(),
		rootPath: rootPath,
	}

	ft.SetBackgroundColor(ui.ColorBg)
	ft.SetGraphicsColor(ui.ColorBorder)
	ft.SetBorder(true)
	ft.SetBorderColor(ui.ColorBorder)
	ft.SetTitle(" Files ")
	ft.SetTitleColor(ui.ColorPanelBlurred)

	root := ft.createNode(rootPath, filepath.Base(rootPath), true)
	ft.SetRoot(root)
	ft.SetCurrentNode(root)

	ft.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		path := ref.(string)
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.IsDir() {
			// Toggle expand/collapse
			if len(node.GetChildren()) > 0 {
				node.SetExpanded(!node.IsExpanded())
			} else {
				ft.addChildren(node, path)
				node.SetExpanded(true)
			}
		} else {
			if ft.onFileOpen != nil {
				ft.onFileOpen(path)
			}
		}
	})

	// Expand root by default
	ft.addChildren(root, rootPath)
	root.SetExpanded(true)

	return ft
}

func (ft *FileTree) SetOnFileOpen(fn func(path string)) {
	ft.onFileOpen = fn
}

func (ft *FileTree) SetRootPath(path string) {
	ft.rootPath = path
	root := ft.createNode(path, filepath.Base(path), true)
	ft.SetRoot(root)
	ft.SetCurrentNode(root)
	ft.addChildren(root, path)
	root.SetExpanded(true)
}

func (ft *FileTree) createNode(path, name string, isDir bool) *tview.TreeNode {
	var icon string
	if isDir {
		icon = ui.Style.DirIconClosed()
	} else {
		icon = ui.Style.FileIcon(name)
	}
	node := tview.NewTreeNode(icon + " " + name)
	node.SetReference(path)
	node.SetSelectable(true)

	if isDir {
		node.SetTextStyle(tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(ui.ColorBg))
		node.SetSelectedTextStyle(tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorTreeSelected))
	} else {
		node.SetTextStyle(tcell.StyleDefault.Foreground(ui.ColorTreeText).Background(ui.ColorBg))
		node.SetSelectedTextStyle(tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorTreeSelected))
	}

	return node
}

func (ft *FileTree) addChildren(parent *tview.TreeNode, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}
		childPath := filepath.Join(path, name)
		child := ft.createNode(childPath, name, entry.IsDir())
		parent.AddChild(child)
	}
}


// RefreshColors re-applies theme colors to the file tree widget and rebuilds nodes.
func (ft *FileTree) RefreshColors() {
	ft.SetBackgroundColor(ui.ColorBg)
	ft.SetGraphicsColor(ui.ColorBorder)
	ft.SetBorderColor(ui.ColorBorder)
	ft.SetTitleColor(ui.ColorPanelBlurred)
	// Recolor all existing nodes without rebuilding (preserves expanded state)
	ft.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		ref := node.GetReference()
		if ref != nil {
			if path, ok := ref.(string); ok {
				info, err := os.Stat(path)
				if err == nil && info.IsDir() {
					node.SetTextStyle(tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(ui.ColorBg))
					node.SetSelectedTextStyle(tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorTreeSelected))
				} else {
					node.SetTextStyle(tcell.StyleDefault.Foreground(ui.ColorTreeText).Background(ui.ColorBg))
					node.SetSelectedTextStyle(tcell.StyleDefault.Foreground(ui.ColorBg).Background(ui.ColorTreeSelected))
				}
			}
		}
		return true
	})
}

// Refresh reloads the file tree
func (ft *FileTree) Refresh() {
	root := ft.createNode(ft.rootPath, filepath.Base(ft.rootPath), true)
	ft.SetRoot(root)
	ft.SetCurrentNode(root)
	ft.addChildren(root, ft.rootPath)
	root.SetExpanded(true)
}

// InputHandler to style selected nodes
func (ft *FileTree) Draw(screen tcell.Screen) {
	ft.TreeView.Draw(screen)
}
