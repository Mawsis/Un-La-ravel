package detector

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type DirectoryNode struct {
	Name     string
	Path     string
	Children []*DirectoryNode
	IsDir    bool
	Parent   *DirectoryNode
}

// LaravelDetector detects if the current directory is a Laravel project.
type LaravelProject struct {
	Path             string
	Version          string
	ComposerAnalysis *ComposerAnalysis
	DirectoryTree    *DirectoryNode
}

func getIgnoredDirectories() []string {
	return []string{".git", "node_modules", "vendor"}
}

func BuildDirectoryTree(rootPath string) (*DirectoryNode, error) {
	var root *DirectoryNode
	nodeMap := make(map[string]*DirectoryNode)

	// Convert to absolute path for consistent comparison
	absRootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking the path %q: %v", path, err)
		}

		if d.IsDir() && slices.Contains(getIgnoredDirectories(), d.Name()) {
			return filepath.SkipDir
		}

		// Convert current path to absolute
		absPath, _ := filepath.Abs(path)

		node := &DirectoryNode{
			Name:  d.Name(),
			Path:  absPath,
			IsDir: d.IsDir(),
		}

		// Check if this is root
		if absPath == absRootPath {
			root = node
			node.Parent = nil
		} else {
			parentPath := filepath.Dir(absPath)

			if parent, exists := nodeMap[parentPath]; exists {
				node.Parent = parent
				parent.Children = append(parent.Children, node)
			}
		}

		nodeMap[absPath] = node
		return nil
	})

	return root, err
}

func DetectLaravel(projectPath string) (*LaravelProject, error) {
	if projectPath == "" {
		projectPath = "."
	}
	// Check if the composer.json file exists
	artisanFile := filepath.Join(projectPath, "artisan")
	if _, err := os.Stat(artisanFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("artisan file not found in %s", projectPath)
	}
	composerAnalysis, err := AnalyzeComposer(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze composer.json: %w", err)
	}
	dirNode, err := BuildDirectoryTree(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build directory tree: %w", err)
	}
	return &LaravelProject{
		Path:             projectPath,
		Version:          composerAnalysis.LaravelVersion,
		ComposerAnalysis: composerAnalysis,
		DirectoryTree:    dirNode,
	}, nil
}
