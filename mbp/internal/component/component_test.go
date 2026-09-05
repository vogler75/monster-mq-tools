package component

import (
	"testing"
)

func TestFindMonsterRoot(t *testing.T) {
	root, err := FindMonsterRoot("")
	if err != nil {
		t.Fatalf("failed to find monster root: %v", err)
	}

	if root == "" {
		t.Fatalf("monster root is empty")
	}

	t.Logf("Detected Monster root: %s", root)

	comps, err := LoadAndRefreshAll(root)
	if err != nil {
		t.Fatalf("failed to load components: %v", err)
	}

	if len(comps) < 4 {
		t.Errorf("expected at least 4 components, got %d", len(comps))
	}

	for _, c := range comps {
		t.Logf("Component [%s]: exists=%v, version=%s (%s), status=%s, git-branch=%s, git-dirty=%v, artifacts=%d",
			c.ID, c.Exists, c.Version, c.VersionSource, c.Status, c.Git.Branch, c.Git.IsDirty, len(c.Artifacts))
		if !c.Exists {
			t.Errorf("expected component %s to exist at %s", c.ID, c.Directory)
		}
	}
}
