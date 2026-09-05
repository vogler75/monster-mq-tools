package git

import (
	"os"
	"testing"
)

func TestGetStatusCurrentRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}

	st := GetStatus(wd)
	if !st.IsRepo {
		t.Fatalf("expected %s to be a git repository", wd)
	}

	if st.Branch == "" {
		t.Errorf("expected branch to not be empty")
	}

	if st.Commit == "" {
		t.Errorf("expected commit to not be empty")
	}

	t.Logf("Repo: %s, Branch: %s, Commit: %s, IsDirty: %v, DirtyCount: %d, Upstream: %s, Ahead: %d, Behind: %d",
		wd, st.Branch, st.Commit, st.IsDirty, st.DirtyFilesCount, st.Upstream, st.Ahead, st.Behind)
}
