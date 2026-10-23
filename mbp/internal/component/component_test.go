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

func TestComponentTargets(t *testing.T) {
	root, err := FindMonsterRoot("")
	if err != nil {
		t.Fatalf("failed to find monster root: %v", err)
	}

	comps := GetStandardRegistry(root)
	toolsComp, err := FindComponent(comps, "tools")
	if err != nil {
		t.Fatalf("tools component not found: %v", err)
	}

	// Verify build targets
	if toolsComp.DefaultBuild.Command == "" {
		t.Errorf("tools component has empty DefaultBuild command")
	}
	if len(toolsComp.BuildTargets) == 0 {
		t.Errorf("tools component has no BuildTargets")
	}

	// Verify publish targets
	if toolsComp.DefaultPublish.Command == "" {
		t.Errorf("tools component has empty DefaultPublish command")
	}
	if len(toolsComp.PublishTargets) == 0 {
		t.Errorf("tools component has no PublishTargets")
	}

	// Verify all components have default build and publish targets
	for _, c := range comps {
		if c.DefaultBuild.Command == "" {
			t.Errorf("component %s has empty DefaultBuild command", c.ID)
		}
		if len(c.BuildTargets) == 0 {
			t.Errorf("component %s has no BuildTargets", c.ID)
		}
		if c.DefaultPublish.Command == "" {
			t.Errorf("component %s has empty DefaultPublish command", c.ID)
		}
		if len(c.PublishTargets) == 0 {
			t.Errorf("component %s has no PublishTargets", c.ID)
		}
		if c.DevVersionTarget != nil {
			if c.DevVersionTarget.Command != "./release.sh" {
				t.Errorf("component %s DevVersionTarget.Command expected './release.sh', got '%s'", c.ID, c.DevVersionTarget.Command)
			}
			if len(c.DevVersionTarget.Args) == 0 || c.DevVersionTarget.Args[0] != "--dev" {
				t.Errorf("component %s DevVersionTarget.Args expected to start with '--dev', got %v", c.ID, c.DevVersionTarget.Args)
			}
		}
	}
}

func TestComponentOrderAndSeparation(t *testing.T) {
	root, err := FindMonsterRoot("")
	if err != nil {
		t.Fatalf("failed to find monster root: %v", err)
	}

	comps := GetStandardRegistry(root)
	if len(comps) == 0 {
		t.Fatalf("no components returned")
	}

	// 1. Dashboard must be on first position
	if comps[0].ID != "dashboard" {
		t.Errorf("expected dashboard at first position (index 0), got %s", comps[0].ID)
	}

	// 2. Explorer and Tools must be separate (excluded from build-all and publish-all)
	for _, c := range comps {
		switch c.ID {
		case "explorer", "tools":
			if !c.IsSeparate() {
				t.Errorf("expected component %s to have IsSeparate() == true", c.ID)
			}
		case "dashboard", "main", "edge":
			if c.IsSeparate() {
				t.Errorf("expected core component %s to have IsSeparate() == false", c.ID)
			}
		}
	}
}
