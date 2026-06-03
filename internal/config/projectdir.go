package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/package-register/mocode/internal/infra/home"
	"github.com/zeebo/xxh3"
)

// ProjectDataDir returns the global per-project data directory.
// It is used as the default DataDirectory when none is configured,
// replacing the old project-local .mocode directory.
//
// Format: <GlobalDataRoot>/projects/<name>_<hash8>/
// Where GlobalDataRoot is the parent directory holding the global mocode.json.
func ProjectDataDir(projectPath string) string {
	return projectDirPath(GlobalDataRoot(), projectPath)
}

// GlobalDataRoot returns the root directory for all global mocode data.
func GlobalDataRoot() string {
	return filepath.Dir(GlobalConfigData())
}

// GlobalStoreDir returns the global data directory where all projects live.
func GlobalStoreDir() string {
	return GlobalDataRoot()
}

// ProjectDirName returns the directory-safe project identifier.
// Format: <base>_<xxh3-hex-prefix8>
func ProjectDirName(projectPath string) string {
	base := filepath.Base(projectPath)
	base = sanitize(base)
	hash := xxh3.HashString(filepath.Clean(projectPath))
	return fmt.Sprintf("%s_%x", base, hash)[:len(base)+1+8] // name + _ + 8 hex chars
}

// projectDirPath returns the full path for a project's data directory.
func projectDirPath(dataDir, projectPath string) string {
	return filepath.Join(dataDir, "projects", ProjectDirName(projectPath))
}

// sanitize replaces characters unsafe for directory names.
func sanitize(name string) string {
	b := make([]byte, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b = append(b, string(r)...)
		} else {
			b = append(b, '-')
		}
	}
	if len(b) == 0 {
		return "project"
	}
	return string(b)
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// GlobalAgentsDirs returns the base .agents/ directories (global scope).
func GlobalAgentsDirs() []string {
	return []string{
		filepath.Join(homeDir(), ".agents"),
		filepath.Join(homeDir(), ".config", "agents"),
	}
}

// ProjectAgentsDirs returns the project-specific .agents/ subdirectories.
func ProjectAgentsDirs(workingDir string) []string {
	return []string{
		filepath.Join(workingDir, ".agents"),
	}
}

// appendUniquePaths appends paths from src to dst if not already present.
func appendUniquePaths(dst, src []string) []string {
	for _, p := range src {
		if !slices.Contains(dst, p) {
			dst = append(dst, p)
		}
	}
	return dst
}

// homeDir returns the path to the user's home directory, using a cached value
// for performance.
func homeDir() string {
	return home.Dir()
}
