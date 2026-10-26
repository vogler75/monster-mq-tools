package component

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindMonsterRoot attempts to resolve the monster root directory (containing main, edge, etc.).
// If overridePath is provided, it is used.
// If MONSTER_ROOT env var is set, it uses it.
// Otherwise it traverses upwards from current working directory until it finds the directory
// containing both "main" and "edge" (or either of them).
func FindMonsterRoot(overridePath string) (string, error) {
	if overridePath != "" {
		abs, err := filepath.Abs(overridePath)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	if envRoot := os.Getenv("MONSTER_ROOT"); envRoot != "" {
		abs, err := filepath.Abs(envRoot)
		if err == nil {
			return abs, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	curr := cwd
	for {
		if isMonsterRoot(curr) {
			return curr, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr || parent == "." || parent == "/" {
			break
		}
		curr = parent
	}

	// Fallback: check ../.. or return cwd
	return cwd, nil
}

func isMonsterRoot(dir string) bool {
	hasMain := dirExists(filepath.Join(dir, "main"))
	hasEdge := dirExists(filepath.Join(dir, "edge"))
	return hasMain && hasEdge
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// GetStandardRegistry returns the list of predefined MonsterMQ components configured for rootDir.
func GetStandardRegistry(rootDir string) []*Component {
	isWin := runtime.GOOS == "windows"
	// Every component has one cross-platform entry point: "make.py <command> [flags]"
	// (the runner starts .py files with a Python 3 interpreter).
	const makePy = "./make.py"

	components := []*Component{
		{
			ID:          "dashboard",
			Name:        "MonsterMQ Dashboard",
			Description: "Web dashboard & cross-platform Electron desktop app (macOS DMG & Windows NSIS)",
			Directory:   filepath.Join(rootDir, "dashboard"),
			VersionFiles: []string{
				"version.txt",
				"package.json",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All",
				Description: "Build web bundle and all desktop packages",
				Command:     makePy,
				Args:        []string{"build", "--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Packages (Web + Desktop)",
					Description: "Build web bundle and all desktop packages",
					Command:     makePy,
					Args:        []string{"build", "--all"},
				},
				{
					ID:          "build-web",
					Name:        "Web Bundle Only",
					Description: "Build web dashboard assets (dist/)",
					Command:     makePy,
					Args:        []string{"build", "--web"},
				},
				{
					ID:          "build-desktop",
					Name:        "Desktop Apps Only",
					Description: "Build macOS DMG & Windows NSIS setup",
					Command:     makePy,
					Args:        []string{"build", "--desktop"},
				},
				{
					ID:          "build-mac",
					Name:        "macOS DMG Only",
					Description: "Build macOS desktop DMG app only",
					Command:     makePy,
					Args:        []string{"build", "--mac"},
				},
				{
					ID:          "build-win",
					Name:        "Windows Setup Only",
					Description: "Build Windows desktop NSIS setup only",
					Command:     makePy,
					Args:        []string{"build", "--win"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean dist/ and dist-desktop/ directories",
				Command:     makePy,
				Args:        []string{"build", "--clean"},
			},
			DevVersionTarget: &Target{
				ID:          "bump-dev",
				Name:        "Prepare Dev Version",
				Description: "Advance to next development version (make.py dev)",
				Command:     makePy,
				Args:        []string{"dev"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Publish desktop apps to GitHub Release",
				Command:     makePy,
				Args:        []string{"publish", "--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All Desktop Packages",
					Description: "Publish macOS DMG and Windows setup to GitHub Release",
					Command:     makePy,
					Args:        []string{"publish", "--all", "-y"},
				},
				{
					ID:          "publish-mac",
					Name:        "Publish macOS DMG Only",
					Description: "Publish macOS DMG package only",
					Command:     makePy,
					Args:        []string{"publish", "--mac", "-y"},
				},
				{
					ID:          "publish-win",
					Name:        "Publish Windows Setup Only",
					Description: "Publish Windows setup package only",
					Command:     makePy,
					Args:        []string{"publish", "--win", "-y"},
				},
			},
			ArtifactGlobs: []string{
				"dist/index.html",
				"dist-desktop/*.dmg",
				"dist-desktop/*-setup.exe",
				"dist-desktop/*.exe",
			},
		},
		{
			ID:          "main",
			Name:        "MonsterMQ Core",
			Description: "Core Enterprise MQTT Broker, Java distribution & Setup binaries",
			Directory:   filepath.Join(rootDir, "main"),
			VersionFiles: []string{
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All (Container)",
				Description: "Build standalone Java broker zip in container, cross-platform setup binaries, and docker image",
				Command:     makePy,
				Args:        []string{"build", "--all", "-c"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Artifacts (Container)",
					Description: "Build broker zip in container, setup executables, and docker image",
					Command:     makePy,
					Args:        []string{"build", "--all", "-c"},
				},
				{
					ID:          "build-broker",
					Name:        "Broker Zip Only (Container)",
					Description: "Build standalone Java broker bundle in container (zip)",
					Command:     makePy,
					Args:        []string{"build", "--broker", "-c"},
				},
				{
					ID:          "build-setup",
					Name:        "Setup Executables Only",
					Description: "Build cross-platform Go setup binaries (setup.exe, setup-mac, etc.)",
					Command:     makePy,
					Args:        []string{"build", "--setup"},
				},
				{
					ID:          "build-docker",
					Name:        "Docker Image Only",
					Description: "Build local Docker image for host platform",
					Command:     makePy,
					Args:        []string{"build", "--docker"},
				},
				{
					ID:          "build-all-local",
					Name:        "All Artifacts (Host/Local)",
					Description: "Build broker zip on host without Docker container",
					Command:     makePy,
					Args:        []string{"build", "--all"},
				},
				{
					ID:          "build-broker-local",
					Name:        "Broker Zip Only (Host/Local)",
					Description: "Build standalone Java broker bundle on host without Docker container",
					Command:     makePy,
					Args:        []string{"build", "--broker"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean output build directories",
				Command:     makePy,
				Args:        []string{"build", "--clean"},
			},
			DevVersionTarget: &Target{
				ID:          "bump-dev",
				Name:        "Prepare Dev Version",
				Description: "Advance to next development version (make.py dev)",
				Command:     makePy,
				Args:        []string{"dev"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Upload release assets to GitHub Release and push multi-arch Docker image",
				Command:     makePy,
				Args:        []string{"publish", "--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All (GitHub + Docker)",
					Description: "Publish all GitHub release assets and push Docker images",
					Command:     makePy,
					Args:        []string{"publish", "--all", "-y"},
				},
				{
					ID:          "publish-broker",
					Name:        "Publish Broker Bundle Only",
					Description: "Publish broker .zip to GitHub Release",
					Command:     makePy,
					Args:        []string{"publish", "--broker", "-y"},
				},
				{
					ID:          "publish-setup",
					Name:        "Publish Setup Binaries Only",
					Description: "Publish setup executables to GitHub Release",
					Command:     makePy,
					Args:        []string{"publish", "--setup", "-y"},
				},
				{
					ID:          "publish-docker",
					Name:        "Publish Docker Hub Only",
					Description: "Build and push multi-arch Docker images to Docker Hub",
					Command:     makePy,
					Args:        []string{"publish", "--docker", "-y"},
				},
			},
			ArtifactGlobs: []string{
				"dist/monstermq-broker-*.zip",
				"dist/setup-*",
				"dist/setup.exe",
			},
		},
		{
			ID:          "edge",
			Name:        "MonsterMQ Edge Broker",
			Description: "Lightweight Go Industrial Edge Broker, Debian packages & Docker",
			Directory:   filepath.Join(rootDir, "edge"),
			VersionFiles: []string{
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All",
				Description: "Build native Go binary, Debian packages, and Docker image",
				Command:     makePy,
				Args:        []string{"build", "--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Artifacts",
					Description: "Build binary, Debian packages, and Docker image",
					Command:     makePy,
					Args:        []string{"build", "--all"},
				},
				{
					ID:          "build-binary",
					Name:        "Native Binary Only",
					Description: "Build native Go binary for current machine",
					Command:     makePy,
					Args:        []string{"build", "--binary"},
				},
				{
					ID:          "build-deb",
					Name:        "Debian Packages Only",
					Description: "Build Debian packages for all architectures (arm64, armhf, amd64)",
					Command:     makePy,
					Args:        []string{"build", "--deb"},
				},
				{
					ID:          "build-docker",
					Name:        "Docker Image Only",
					Description: "Build local Docker image for host platform",
					Command:     makePy,
					Args:        []string{"build", "--docker"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean build output directories",
				Command:     makePy,
				Args:        []string{"build", "--clean"},
			},
			DevVersionTarget: &Target{
				ID:          "bump-dev",
				Name:        "Prepare Dev Version",
				Description: "Advance to next development version (make.py dev)",
				Command:     makePy,
				Args:        []string{"dev"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Publish Debian packages to GitHub Release and push Docker image",
				Command:     makePy,
				Args:        []string{"publish", "--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All (GitHub + Docker)",
					Description: "Publish Debian packages to GitHub and push Docker image",
					Command:     makePy,
					Args:        []string{"publish", "--all", "-y"},
				},
				{
					ID:          "publish-github",
					Name:        "Publish GitHub Release Only",
					Description: "Upload Debian packages to GitHub Release only",
					Command:     makePy,
					Args:        []string{"publish", "--github", "-y"},
				},
				{
					ID:          "publish-docker",
					Name:        "Publish Docker Hub Only",
					Description: "Build and push multi-arch Docker image to Docker Hub",
					Command:     makePy,
					Args:        []string{"publish", "--docker", "-y"},
				},
			},
			ArtifactGlobs: []string{
				"bin/monstermq-edge*",
				"bin/*.deb",
			},
		},
		{
			ID:          "explorer",
			Name:        "MonsterMQ Explorer",
			Description: "Electron desktop MQTT client & topic explorer (macOS & Windows)",
			Directory:   filepath.Join(rootDir, "explorer"),
			Separate:    true,
			// version.txt carries the -dev version; package.json only follows release tags
			VersionFiles: []string{
				"version.txt",
				"package.json",
			},
			DefaultBuild: defaultExplorerBuild(isWin),
			BuildTargets: explorerBuildTargets(isWin),
			DevVersionTarget: &Target{
				ID:          "bump-dev",
				Name:        "Prepare Dev Version",
				Description: "Advance to next development version (make.py dev)",
				Command:     makePy,
				Args:        []string{"dev"},
			},
			DefaultPublish: defaultExplorerPublish(isWin),
			PublishTargets: explorerPublishTargets(isWin),
			ArtifactGlobs: []string{
				"release/*.dmg",
				"release/*.exe",
				"release/*.zip",
				"dist/index.html",
			},
		},
		{
			ID:          "tools",
			Name:        "MonsterMQ Tools",
			Description: "CLI tools (mmq, i3x, gql schema tools)",
			Directory:   filepath.Join(rootDir, "tools"),
			Separate:    true,
			VersionFiles: []string{
				"version.txt",
				"cli/version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-native",
				Name:        "Build Native Tools",
				Description: "Build native binaries for all tools (mmq, i3x, gql, mbp)",
				Command:     makePy,
				Args:        []string{"build", "--native"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-native",
					Name:        "Build Native Tools",
					Description: "Build native binaries for all tools (mmq, i3x, gql, mbp)",
					Command:     makePy,
					Args:        []string{"build", "--native"},
				},
				{
					ID:          "build-all",
					Name:        "Cross-compile All Tools",
					Description: "Cross-compile all tools for linux, darwin, and windows",
					Command:     makePy,
					Args:        []string{"build", "--all"},
				},
				{
					ID:          "build-cli",
					Name:        "Build mmq CLI Only",
					Description: "Build MonsterMQ CLI native binary only",
					Command:     makePy,
					Args:        []string{"build", "--cli"},
				},
				{
					ID:          "build-i3x",
					Name:        "Build i3x CLI Only",
					Description: "Build i3x CLI native binary only",
					Command:     makePy,
					Args:        []string{"build", "--i3x"},
				},
				{
					ID:          "build-mbp",
					Name:        "Build mbp Pipeline Only",
					Description: "Build mbp pipeline binary only",
					Command:     makePy,
					Args:        []string{"build", "--mbp"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean Tools",
				Description: "Clean all tools bin/ directories",
				Command:     makePy,
				Args:        []string{"build", "--clean"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All Tools",
				Description: "Upload release binaries for all tools to GitHub Release",
				Command:     makePy,
				Args:        []string{"publish", "--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All Tools",
					Description: "Upload release binaries for all tools to GitHub Release",
					Command:     makePy,
					Args:        []string{"publish", "--all", "-y"},
				},
				{
					ID:          "publish-build-all",
					Name:        "Build & Publish All Tools",
					Description: "Cross-compile all tools first and upload to GitHub Release",
					Command:     makePy,
					Args:        []string{"publish", "--build", "-y"},
				},
			},
			ArtifactGlobs: []string{
				"cli/bin/mmq*",
				"i3x/bin/i3x*",
				"mbp/bin/mbp*",
				"gql/bin/*",
			},
		},
		{
			ID:          "tests",
			Name:        "MonsterMQ Tests",
			Description: "Test programs (mqtt-compliance suite, mq-bench benchmark)",
			Directory:   filepath.Join(rootDir, "tests"),
			Separate:    true,
			VersionFiles: []string{
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All Test Programs",
				Description: "Build mqtt-compliance and mq-bench binaries",
				Command:     makePy,
				Args:        []string{"build", "--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "Build All Test Programs",
					Description: "Build mqtt-compliance and mq-bench binaries",
					Command:     makePy,
					Args:        []string{"build", "--all"},
				},
				{
					ID:          "build-test",
					Name:        "Build & Unit Test All",
					Description: "Build all test programs and run their unit tests",
					Command:     makePy,
					Args:        []string{"build", "--all", "--test"},
				},
				{
					ID:          "build-compliance",
					Name:        "Build mqtt-compliance Only",
					Description: "Build MQTT compliance suite binary only",
					Command:     makePy,
					Args:        []string{"build", "--compliance"},
				},
				{
					ID:          "build-performance",
					Name:        "Build mq-bench Only",
					Description: "Build multi-protocol benchmark binary only",
					Command:     makePy,
					Args:        []string{"build", "--performance"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean Test Programs",
				Description: "Remove mqtt-compliance and mq-bench binaries",
				Command:     makePy,
				Args:        []string{"build", "--clean"},
			},
			ArtifactGlobs: []string{
				"compliance/mqtt-compliance",
				"compliance/mqtt-compliance.exe",
				"performance/mq-bench",
				"performance/mq-bench.exe",
			},
		},
	}

	return components
}

func defaultExplorerBuild(isWin bool) Target {
	if isWin {
		return Target{
			ID:          "build-win",
			Name:        "Windows Setup Only",
			Description: "Build Windows Electron EXE installer",
			Command:     "./make.py",
			Args:        []string{"build", "--win"},
		}
	}
	return Target{
		ID:          "build-all",
		Name:        "Build All Platforms",
		Description: "Build macOS, Windows and Linux Electron app packages",
		Command:     "./make.py",
		Args:        []string{"build", "--all"},
	}
}

func explorerBuildTargets(isWin bool) []Target {
	targets := []Target{
		{
			ID:          "build-win",
			Name:        "Windows Setup Only",
			Description: "Build Windows Electron EXE installer",
			Command:     "./make.py",
			Args:        []string{"build", "--win"},
		},
		{
			ID:          "build-pwa",
			Name:        "PWA Web Bundle",
			Description: "Build production PWA bundle (dist/)",
			Command:     "./make.py",
			Args:        []string{"build", "--pwa"},
		},
		{
			ID:          "build-docker",
			Name:        "Docker Image",
			Description: "Build Explorer web bundle in a Docker container",
			Command:     "./make.py",
			Args:        []string{"build", "--docker"},
		},
	}
	if isWin {
		return targets
	}
	return append([]Target{
		{
			ID:          "build-all",
			Name:        "All Platforms",
			Description: "Build macOS, Windows and Linux Electron app packages",
			Command:     "./make.py",
			Args:        []string{"build", "--all"},
		},
		{
			ID:          "build-mac",
			Name:        "macOS App Only",
			Description: "Build macOS Electron packages",
			Command:     "./make.py",
			Args:        []string{"build", "--mac"},
		},
	}, targets...)
}

func defaultExplorerPublish(isWin bool) Target {
	return Target{
		ID:          "publish",
		Name:        "Publish GitHub Release",
		Description: "Upload release artifacts in release/ to GitHub Release via gh CLI",
		Command:     "./make.py",
		Args:        []string{"publish"},
	}
}

func explorerPublishTargets(isWin bool) []Target {
	return []Target{defaultExplorerPublish(isWin)}
}

// LoadAndRefreshAll discovers and loads all components with up-to-date statuses.
func LoadAndRefreshAll(rootDir string) ([]*Component, error) {
	comps := GetStandardRegistry(rootDir)
	for _, c := range comps {
		RefreshStatus(c)
	}
	return comps, nil
}

// FindComponent finds a component by ID in the list.
func FindComponent(comps []*Component, id string) (*Component, error) {
	for _, c := range comps {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("component '%s' not found", id)
}
