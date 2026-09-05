package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Status represents the git state of a repository.
type Status struct {
	IsRepo          bool      `json:"is_repo"`
	Branch          string    `json:"branch"`
	Commit          string    `json:"commit"`
	CommitTime      time.Time `json:"commit_time"`
	IsDirty         bool      `json:"is_dirty"`
	DirtyFilesCount int       `json:"dirty_files_count"`
	Upstream        string    `json:"upstream"`
	HasUpstream     bool      `json:"has_upstream"`
	Ahead           int       `json:"ahead"`
	Behind          int       `json:"behind"`
	Error           string    `json:"error,omitempty"`
}

// GetStatus inspects the git repository at repoDir and returns its current Status.
func GetStatus(repoDir string) Status {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return GetStatusWithContext(ctx, repoDir)
}

// GetStatusWithContext inspects the git repository with a context timeout.
func GetStatusWithContext(ctx context.Context, repoDir string) Status {
	st := Status{IsRepo: false}

	// 1. Check if git repo
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		st.Error = "not a git repository"
		return st
	}
	st.IsRepo = true

	// 2. Current branch
	if branchOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		st.Branch = strings.TrimSpace(string(branchOut))
	}

	// 3. Short commit hash
	if commitOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		st.Commit = strings.TrimSpace(string(commitOut))
	}

	// 4. Commit timestamp (Unix seconds)
	if timeOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "log", "-1", "--format=%ct").Output(); err == nil {
		if sec, err := strconv.ParseInt(strings.TrimSpace(string(timeOut)), 10, 64); err == nil {
			st.CommitTime = time.Unix(sec, 0)
		}
	}

	// 5. Dirty check
	if statusOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "status", "--porcelain").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
		count := 0
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				count++
			}
		}
		st.DirtyFilesCount = count
		st.IsDirty = count > 0
	}

	// 6. Upstream tracking branch
	if upOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--abbrev-ref", "@{u}").Output(); err == nil {
		st.Upstream = strings.TrimSpace(string(upOut))
		st.HasUpstream = st.Upstream != ""
	}

	// 7. Ahead / Behind counts
	if st.HasUpstream {
		if countOut, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-list", "--left-right", "--count", "HEAD...@{u}").Output(); err == nil {
			parts := strings.Fields(strings.TrimSpace(string(countOut)))
			if len(parts) >= 2 {
				ahead, _ := strconv.Atoi(parts[0])
				behind, _ := strconv.Atoi(parts[1])
				st.Ahead = ahead
				st.Behind = behind
			}
		}
	}

	return st
}

// Fetch executes git fetch for the repoDir to update remote tracking branches.
func Fetch(ctx context.Context, repoDir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "--prune")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
