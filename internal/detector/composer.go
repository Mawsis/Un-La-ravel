package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ComposerData struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev,omitempty"`
	Name       string            `json:"name,omitempty"`
}

type ComposerInfo struct {
	Name          string
	Version       string
	IsLaravelCore bool
}

type ComposerAnalysis struct {
	LaravelVersion  string
	Dependencies    []ComposerInfo
	DevDependencies []ComposerInfo
	ProjectName     string
}

func AnalyzeComposer(projectPath string) (*ComposerAnalysis, error) {
	composerFile := filepath.Join(projectPath, "composer.json")
	if _, err := os.Stat(composerFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("composer.json not found in %s", projectPath)
	}

	composerData, err := os.ReadFile(composerFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read composer.json: %w", err)
	}
	var composer ComposerData
	if err := json.Unmarshal(composerData, &composer); err != nil {
		return nil, fmt.Errorf("failed to parse composer.json: %w", err)
	}
	var composerAnalysis ComposerAnalysis
	for req := range composer.Require {
		var info ComposerInfo = ComposerInfo{
			Name:          req,
			Version:       composer.Require[req],
			IsLaravelCore: CheckIfLaravelCore(req),
		}
		if info.Name == "laravel/framework" {
			composerAnalysis.LaravelVersion = info.Version
		}
		composerAnalysis.Dependencies = append(composerAnalysis.Dependencies, info)
	}
	for req := range composer.RequireDev {
		var info ComposerInfo = ComposerInfo{
			Name:          req,
			Version:       composer.RequireDev[req],
			IsLaravelCore: CheckIfLaravelCore(req),
		}
		composerAnalysis.DevDependencies = append(composerAnalysis.DevDependencies, info)
	}
	if composerAnalysis.LaravelVersion == "" {
		return nil, fmt.Errorf("laravel/framework not found in composer.json")
	}
	composerAnalysis.ProjectName = composer.Name
	return &composerAnalysis, nil
}

func CheckIfLaravelCore(packageName string) bool {
	//Check if starts with laravel/
	if len(packageName) >= 8 && packageName[:8] == "laravel/" {
		return true
	}
	return false
}
