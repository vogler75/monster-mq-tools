package component

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vogler75/monster-mq-tools/mbp/internal/git"
)

// DetectVersion reads the version string from candidate files in repoDir.
func DetectVersion(repoDir string, candidateFiles []string) (string, string) {
	for _, rel := range candidateFiles {
		fullPath := filepath.Join(repoDir, rel)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		if strings.HasSuffix(rel, ".json") {
			var pkg struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(data, &pkg); err == nil && pkg.Version != "" {
				return pkg.Version, rel
			}
		} else {
			// e.g. version.txt
			firstLine := strings.TrimSpace(strings.Split(string(data), "\n")[0])
			firstLine = strings.TrimSpace(strings.TrimPrefix(firstLine, "v"))
			// Strip build metadata after '+' if any (e.g. 1.8.32+123 -> 1.8.32)
			if idx := strings.Index(firstLine, "+"); idx != -1 {
				firstLine = firstLine[:idx]
			}
			if firstLine != "" {
				return firstLine, rel
			}
		}
	}
	return "unknown", "none"
}

// FindArtifacts searches for built artifact files using glob patterns relative to repoDir.
func FindArtifacts(repoDir string, globs []string) []ArtifactInfo {
	var results []ArtifactInfo
	seen := make(map[string]bool)

	for _, g := range globs {
		pattern := filepath.Join(repoDir, g)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			if seen[match] {
				continue
			}
			fi, err := os.Stat(match)
			if err != nil || fi.IsDir() {
				continue
			}

			// Exclude blockmaps and temporary files
			name := fi.Name()
			if strings.HasSuffix(name, ".blockmap") || strings.HasSuffix(name, ".tmp") {
				continue
			}

			seen[match] = true
			rel, _ := filepath.Rel(repoDir, match)
			results = append(results, ArtifactInfo{
				Name:    name,
				Path:    rel,
				Size:    fi.Size(),
				ModTime: fi.ModTime(),
			})
		}
	}

	return results
}

// RefreshStatus updates the Git status, version, artifacts, and build status of a component.
func RefreshStatus(c *Component) {
	fi, err := os.Stat(c.Directory)
	if err != nil || !fi.IsDir() {
		c.Exists = false
		c.Status = StatusNotBuilt
		return
	}
	c.Exists = true

	// 1. Detect Version
	v, src := DetectVersion(c.Directory, c.VersionFiles)
	c.Version = v
	c.VersionSource = src

	// 2. Git Status
	c.Git = git.GetStatus(c.Directory)

	// 3. Artifact Discovery
	artifacts := FindArtifacts(c.Directory, c.ArtifactGlobs)
	c.Artifacts = artifacts

	var latest *ArtifactInfo
	for i := range artifacts {
		if latest == nil || artifacts[i].ModTime.After(latest.ModTime) {
			latest = &artifacts[i]
		}
	}
	c.LatestArtifact = latest

	// 4. Calculate Build Status
	if len(artifacts) == 0 {
		c.Status = StatusNotBuilt
		return
	}

	// If artifacts exist, compare with git commit time
	if c.Git.IsRepo && !c.Git.CommitTime.IsZero() {
		// If last git commit is strictly newer than artifact, it's outdated
		// Add a slight 2-second grace period for clock skew
		if c.Git.CommitTime.After(latest.ModTime.Add(2 * time.Second)) {
			c.Status = StatusOutdated
			return
		}
	}

	// If dirty working tree has modified source code, consider it outdated or dirty
	if c.Git.IsDirty {
		c.Status = StatusOutdated
		return
	}

	c.Status = StatusBuilt
}
