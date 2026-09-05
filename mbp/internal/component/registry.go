package component

import (
	"fmt"
	"os"
	"path/filepath"
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
	components := []*Component{
		{
			ID:          "main",
			Name:        "MonsterMQ Main Broker",
			Description: "Core Enterprise MQTT Broker, Java distribution & Setup binaries",
			Directory:   filepath.Join(rootDir, "main"),
			VersionFiles: []string{
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All",
				Description: "Build standalone Java broker zip, cross-platform setup binaries, and docker image",
				Command:     "./build.sh",
				Args:        []string{"--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Artifacts",
					Description: "Build broker zip, setup executables, and docker image",
					Command:     "./build.sh",
					Args:        []string{"--all"},
				},
				{
					ID:          "build-broker",
					Name:        "Broker Zip Only",
					Description: "Build standalone Java broker bundle (zip)",
					Command:     "./build.sh",
					Args:        []string{"--broker"},
				},
				{
					ID:          "build-setup",
					Name:        "Setup Executables Only",
					Description: "Build cross-platform Go setup binaries (setup.exe, setup-mac, etc.)",
					Command:     "./build.sh",
					Args:        []string{"--setup"},
				},
				{
					ID:          "build-docker",
					Name:        "Docker Image Only",
					Description: "Build local Docker image for host platform",
					Command:     "./build.sh",
					Args:        []string{"--docker"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean output build directories",
				Command:     "./build.sh",
				Args:        []string{"--clean"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Upload release assets to GitHub Release and push multi-arch Docker image",
				Command:     "./publish.sh",
				Args:        []string{"--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All (GitHub + Docker)",
					Description: "Publish all GitHub release assets and push Docker images",
					Command:     "./publish.sh",
					Args:        []string{"--all", "-y"},
				},
				{
					ID:          "publish-broker",
					Name:        "Publish Broker Bundle Only",
					Description: "Publish broker .zip to GitHub Release",
					Command:     "./publish.sh",
					Args:        []string{"--broker", "-y"},
				},
				{
					ID:          "publish-setup",
					Name:        "Publish Setup Binaries Only",
					Description: "Publish setup executables to GitHub Release",
					Command:     "./publish.sh",
					Args:        []string{"--setup", "-y"},
				},
				{
					ID:          "publish-docker",
					Name:        "Publish Docker Hub Only",
					Description: "Build and push multi-arch Docker images to Docker Hub",
					Command:     "./publish.sh",
					Args:        []string{"--docker", "-y"},
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
				Command:     "./build.sh",
				Args:        []string{"--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Artifacts",
					Description: "Build binary, Debian packages, and Docker image",
					Command:     "./build.sh",
					Args:        []string{"--all"},
				},
				{
					ID:          "build-binary",
					Name:        "Native Binary Only",
					Description: "Build native Go binary for current machine",
					Command:     "./build.sh",
					Args:        []string{"--binary"},
				},
				{
					ID:          "build-deb",
					Name:        "Debian Packages Only",
					Description: "Build Debian packages for all architectures (arm64, armhf, amd64)",
					Command:     "./build.sh",
					Args:        []string{"--deb"},
				},
				{
					ID:          "build-docker",
					Name:        "Docker Image Only",
					Description: "Build local Docker image for host platform",
					Command:     "./build.sh",
					Args:        []string{"--docker"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean build output directories",
				Command:     "./build.sh",
				Args:        []string{"--clean"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Publish Debian packages to GitHub Release and push Docker image",
				Command:     "./publish.sh",
				Args:        []string{"--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All (GitHub + Docker)",
					Description: "Publish Debian packages to GitHub and push Docker image",
					Command:     "./publish.sh",
					Args:        []string{"--all", "-y"},
				},
				{
					ID:          "publish-github",
					Name:        "Publish GitHub Release Only",
					Description: "Upload Debian packages to GitHub Release only",
					Command:     "./publish.sh",
					Args:        []string{"--github", "-y"},
				},
				{
					ID:          "publish-docker",
					Name:        "Publish Docker Hub Only",
					Description: "Build and push multi-arch Docker image to Docker Hub",
					Command:     "./publish.sh",
					Args:        []string{"--docker", "-y"},
				},
			},
			ArtifactGlobs: []string{
				"bin/monstermq-edge",
				"bin/monstermq-edge-linux-*",
				"bin/*.deb",
			},
		},
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
				Command:     "./build.sh",
				Args:        []string{"--all"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Packages (Web + Desktop)",
					Description: "Build web bundle and all desktop packages",
					Command:     "./build.sh",
					Args:        []string{"--all"},
				},
				{
					ID:          "build-web",
					Name:        "Web Bundle Only",
					Description: "Build web dashboard assets (dist/)",
					Command:     "./build.sh",
					Args:        []string{"--web"},
				},
				{
					ID:          "build-desktop",
					Name:        "Desktop Apps Only",
					Description: "Build macOS DMG & Windows NSIS setup",
					Command:     "./build.sh",
					Args:        []string{"--desktop"},
				},
				{
					ID:          "build-mac",
					Name:        "macOS DMG Only",
					Description: "Build macOS desktop DMG app only",
					Command:     "./build.sh",
					Args:        []string{"--mac"},
				},
				{
					ID:          "build-win",
					Name:        "Windows Setup Only",
					Description: "Build Windows desktop NSIS setup only",
					Command:     "./build.sh",
					Args:        []string{"--win"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean",
				Description: "Clean dist/ and dist-desktop/ directories",
				Command:     "./build.sh",
				Args:        []string{"--clean"},
			},
			DefaultPublish: Target{
				ID:          "publish-all",
				Name:        "Publish All",
				Description: "Publish desktop apps to GitHub Release",
				Command:     "./publish.sh",
				Args:        []string{"--all", "-y"},
			},
			PublishTargets: []Target{
				{
					ID:          "publish-all",
					Name:        "Publish All Desktop Packages",
					Description: "Publish macOS DMG and Windows setup to GitHub Release",
					Command:     "./publish.sh",
					Args:        []string{"--all", "-y"},
				},
				{
					ID:          "publish-mac",
					Name:        "Publish macOS DMG Only",
					Description: "Publish macOS DMG package only",
					Command:     "./publish.sh",
					Args:        []string{"--mac", "-y"},
				},
				{
					ID:          "publish-win",
					Name:        "Publish Windows Setup Only",
					Description: "Publish Windows setup package only",
					Command:     "./publish.sh",
					Args:        []string{"--win", "-y"},
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
			ID:          "explorer",
			Name:        "MonsterMQ Explorer",
			Description: "Electron desktop MQTT client & topic explorer (macOS & Windows)",
			Directory:   filepath.Join(rootDir, "explorer"),
			VersionFiles: []string{
				"package.json",
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-all",
				Name:        "Build All Platforms",
				Description: "Build macOS and Windows Electron app packages",
				Command:     "./build.sh",
				Args:        []string{},
			},
			BuildTargets: []Target{
				{
					ID:          "build-all",
					Name:        "All Platforms",
					Description: "Build macOS and Windows Electron app packages",
					Command:     "./build.sh",
					Args:        []string{},
				},
				{
					ID:          "build-mac",
					Name:        "macOS App Only",
					Description: "Build macOS Electron DMG packages",
					Command:     "./build-mac.sh",
					Args:        []string{},
				},
				{
					ID:          "build-win",
					Name:        "Windows Setup Only",
					Description: "Build Windows Electron EXE installer",
					Command:     "./build-win.sh",
					Args:        []string{},
				},
				{
					ID:          "build-pwa",
					Name:        "PWA Web Bundle",
					Description: "Build production PWA bundle (dist/)",
					Command:     "./build-pwa.sh",
					Args:        []string{},
				},
				{
					ID:          "build-docker",
					Name:        "Docker Image",
					Description: "Build Explorer web docker container",
					Command:     "./build-docker.sh",
					Args:        []string{},
				},
			},
			DefaultPublish: Target{
				ID:          "publish",
				Name:        "Publish GitHub Release",
				Description: "Upload release artifacts in release/ to GitHub Release via gh CLI",
				Command:     "./publish.sh",
				Args:        []string{},
			},
			PublishTargets: []Target{
				{
					ID:          "publish",
					Name:        "Publish GitHub Release",
					Description: "Upload release artifacts to GitHub Release",
					Command:     "./publish.sh",
					Args:        []string{},
				},
			},
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
			VersionFiles: []string{
				"cli/version.txt",
				"version.txt",
			},
			DefaultBuild: Target{
				ID:          "build-native",
				Name:        "Build Native Tools",
				Description: "Build native mmq and i3x CLI binaries",
				Command:     "bash",
				Args:        []string{"-c", "cd cli && ./build.sh --native && cd ../i3x && ./build.sh"},
			},
			BuildTargets: []Target{
				{
					ID:          "build-native",
					Name:        "Build Native Tools",
					Description: "Build native mmq and i3x CLI binaries",
					Command:     "bash",
					Args:        []string{"-c", "cd cli && ./build.sh --native && cd ../i3x && ./build.sh"},
				},
				{
					ID:          "build-all",
					Name:        "Cross-compile All Tools",
					Description: "Cross-compile mmq for linux, darwin, and windows",
					Command:     "bash",
					Args:        []string{"-c", "cd cli && ./build.sh --all"},
				},
			},
			CleanTarget: &Target{
				ID:          "clean",
				Name:        "Clean Tools",
				Description: "Clean cli/bin and i3x/bin directories",
				Command:     "bash",
				Args:        []string{"-c", "rm -rf cli/bin i3x/bin"},
			},
			ArtifactGlobs: []string{
				"cli/bin/mmq*",
				"i3x/bin/i3x*",
			},
		},
	}

	return components
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
