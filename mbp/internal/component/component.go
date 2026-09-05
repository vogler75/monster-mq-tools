package component

import (
	"time"

	"github.com/vogler75/monster-mq-tools/mbp/internal/git"
)

// BuildStatus represents the compilation / build state of a component.
type BuildStatus string

const (
	StatusNotBuilt   BuildStatus = "Not Built"
	StatusBuilt      BuildStatus = "Built"
	StatusOutdated   BuildStatus = "Outdated"
	StatusBuilding   BuildStatus = "Building"
	StatusPublishing BuildStatus = "Publishing"
	StatusSuccess    BuildStatus = "Success"
	StatusFailed     BuildStatus = "Failed"
)

// Target defines an executable build or publish target option.
type Target struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
}

// ArtifactInfo holds details about a compiled artifact on disk.
type ArtifactInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Component represents a MonsterMQ sub-system or repository.
type Component struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Directory   string   `json:"directory"` // Absolute path
	Exists      bool     `json:"exists"`

	VersionFiles    []string `json:"version_files"`
	Version         string   `json:"version"`
	VersionSource   string   `json:"version_source"`

	BuildTargets    []Target `json:"build_targets"`
	DefaultBuild    Target   `json:"default_build"`

	PublishTargets  []Target `json:"publish_targets"`
	DefaultPublish  Target   `json:"default_publish"`

	CleanTarget     *Target  `json:"clean_target,omitempty"`

	ArtifactGlobs   []string `json:"artifact_globs"`
	Artifacts       []ArtifactInfo `json:"artifacts"`
	LatestArtifact  *ArtifactInfo  `json:"latest_artifact,omitempty"`

	Git             git.Status  `json:"git"`
	Status          BuildStatus `json:"status"`

	LastBuildTime     time.Time     `json:"last_build_time,omitempty"`
	LastBuildDuration time.Duration `json:"last_build_duration,omitempty"`
	LastError         string        `json:"last_error,omitempty"`
}

// HasUpdatesFromGit returns true if the remote git repository has new commits available.
func (c *Component) HasUpdatesFromGit() bool {
	return c.Git.IsRepo && c.Git.HasUpstream && c.Git.Behind > 0
}

// IsUpToDate returns true if git is clean, no commits behind remote, and artifacts are fresh.
func (c *Component) IsUpToDate() bool {
	if !c.Git.IsRepo {
		return c.Status == StatusBuilt
	}
	return !c.Git.IsDirty && c.Git.Behind == 0 && c.Status == StatusBuilt
}
