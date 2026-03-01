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
	ft.SetBorder(false)
	ft.SetTitle(" Files ")
	ft.SetTitleColor(ui.ColorTextWhite)

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
	icon := ft.fileIcon(name, isDir)
	node := tview.NewTreeNode(icon + " " + name)
	node.SetReference(path)
	node.SetSelectable(true)
	node.SetColor(ui.ColorTreeText)

	if isDir {
		node.SetColor(ui.ColorTextWhite)
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

func (ft *FileTree) fileIcon(name string, isDir bool) string {
	if isDir {
		return "+"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "g"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "c"
	case ".py":
		return "p"
	case ".rs":
		return "r"
	case ".js", ".jsx":
		return "j"
	case ".ts", ".tsx":
		return "t"
	case ".java":
		return "j"
	case ".json":
		return "~"
	case ".md":
		return "m"
	case ".html", ".htm":
		return "h"
	case ".css":
		return "#"
	case ".sh", ".bash":
		return "$"
	case ".yaml", ".yml":
		return "y"
	default:
		return "-"
	}
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
