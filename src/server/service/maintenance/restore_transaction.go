// SPDX-License-Identifier: MIT
package maintenance

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	projectpath "github.com/apimgr/vidveil/src/path"
)

type restoreRootStage struct {
	target    string
	staged    string
	old       string
	hadTarget bool
	installed bool
}

func restoreFilesAtomically(files []restoreFileEntry, roots map[string]string) (retErr error) {
	stages := make(map[string]*restoreRootStage)
	order := []string{"config", "data", "ssl"}
	committed := false
	if err := validateRestoreRoots(roots); err != nil {
		return err
	}
	cleanupStages := func() error {
		var cleanupErr error
		for _, stage := range stages {
			if stage.staged != "" {
				if err := os.RemoveAll(stage.staged); err != nil {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove staged restore %q: %w", stage.staged, err))
				}
			}
		}
		return cleanupErr
	}
	defer func() {
		if retErr != nil && !committed {
			for i := len(order) - 1; i >= 0; i-- {
				stage := stages[order[i]]
				if stage == nil || !stage.installed {
					continue
				}
				if err := os.RemoveAll(stage.target); err != nil {
					retErr = errors.Join(retErr, fmt.Errorf("remove partially restored path %q: %w", stage.target, err))
					continue
				}
				if stage.hadTarget {
					if err := os.Rename(stage.old, stage.target); err != nil {
						retErr = errors.Join(retErr, fmt.Errorf("rollback restore of %q: %w", stage.target, err))
						continue
					}
					stage.old = ""
				}
				stage.installed = false
			}
		}
		retErr = errors.Join(retErr, cleanupStages())
	}()

	for _, file := range files {
		rootName := ""
		relative := ""
		for _, name := range order {
			prefix := name + "/"
			if strings.HasPrefix(file.name, prefix) {
				rootName = name
				relative = strings.TrimPrefix(file.name, prefix)
				break
			}
		}
		if rootName == "" {
			continue
		}
		stageName := rootName
		if rootName == "ssl" && pathWithin(roots["config"], roots["ssl"]) && !sameRestorePath(roots["config"], roots["ssl"]) {
			stageName = "config"
			sslRelative, err := filepath.Rel(roots["config"], roots["ssl"])
			if err != nil {
				return fmt.Errorf("resolve SSL restore location: %w", err)
			}
			relative = filepath.Join(sslRelative, filepath.FromSlash(relative))
		}
		stage := stages[stageName]
		if stage == nil {
			target := roots[stageName]
			parent := filepath.Dir(target)
			if err := os.MkdirAll(parent, 0755); err != nil {
				return fmt.Errorf("create restore parent %q: %w", parent, err)
			}
			stageDir, err := os.MkdirTemp(parent, ".vidveil-restore-stage-")
			if err != nil {
				return fmt.Errorf("create staging directory for %s: %w", stageName, err)
			}
			stage = &restoreRootStage{target: target, staged: stageDir}
			stages[stageName] = stage
		}

		targetPath, err := resolveRestoreTarget(stage.staged, relative)
		if err != nil {
			return fmt.Errorf("refusing to stage %q: %w", file.name, err)
		}
		if file.isDir {
			if relative == "" {
				if err := os.Chmod(targetPath, 0700); err != nil {
					return fmt.Errorf("set staged root permissions: %w", err)
				}
				continue
			}
			if err := os.MkdirAll(targetPath, 0700); err != nil {
				return fmt.Errorf("create staged directory: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			return fmt.Errorf("create staged parent: %w", err)
		}
		if err := ensureRestorePathHasNoSymlinks(stage.staged, targetPath); err != nil {
			return fmt.Errorf("refusing to stage %q: %w", file.name, err)
		}
		if file.contentPath == "" {
			if err := os.WriteFile(targetPath, file.content, 0600); err != nil {
				return fmt.Errorf("write staged file %q: %w", file.name, err)
			}
			continue
		}
		if err := copyRestoreFile(file.contentPath, targetPath, 0600); err != nil {
			return fmt.Errorf("write staged file %q: %w", file.name, err)
		}
	}

	for _, name := range order {
		stage := stages[name]
		if stage == nil {
			continue
		}
		if err := ensureRestorePathHasNoSymlinks(filepath.Dir(stage.target), stage.target); err != nil {
			return fmt.Errorf("unsafe restore destination %q: %w", stage.target, err)
		}
		info, err := os.Lstat(stage.target)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("restore destination %q is not a real directory", stage.target)
			}
			oldDir, err := os.MkdirTemp(filepath.Dir(stage.target), ".vidveil-restore-old-")
			if err != nil {
				return fmt.Errorf("prepare rollback for %q: %w", stage.target, err)
			}
			if err := os.Remove(oldDir); err != nil {
				return fmt.Errorf("prepare rollback path: %w", err)
			}
			if err := os.Rename(stage.target, oldDir); err != nil {
				return fmt.Errorf("move current restore destination: %w", err)
			}
			stage.old = oldDir
			stage.hadTarget = true
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect restore destination %q: %w", stage.target, err)
		}
		if err := os.Rename(stage.staged, stage.target); err != nil {
			if stage.hadTarget {
				if rollbackErr := os.Rename(stage.old, stage.target); rollbackErr != nil {
					return errors.Join(fmt.Errorf("install restored %s: %w", name, err), fmt.Errorf("restore previous %s: %w", name, rollbackErr))
				}
				stage.old = ""
			}
			return fmt.Errorf("install restored %s: %w", name, err)
		}
		stage.staged = ""
		stage.installed = true
		if err := applyRestoredPermissions(stage.target, projectpath.IsPrivilegedMode()); err != nil {
			return fmt.Errorf("set restored %s permissions: %w", name, err)
		}
	}
	committed = true

	for i := len(order) - 1; i >= 0; i-- {
		stage := stages[order[i]]
		if stage == nil || !stage.installed || !stage.hadTarget {
			continue
		}
		if err := os.RemoveAll(stage.old); err != nil {
			return fmt.Errorf("remove replaced restore data %q: %w", stage.old, err)
		}
		stage.old = ""
	}
	return nil
}

func applyRestoredPermissions(root string, privileged bool) error {
	dirMode := os.FileMode(0700)
	fileMode := os.FileMode(0600)
	if privileged {
		dirMode = 0755
		fileMode = 0644
	}
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("restored path %q contains a symbolic link", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			directories = append(directories, path)
			return nil
		}
		return os.Chmod(path, fileMode)
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i > 0; i-- {
		if err := os.Chmod(directories[i], dirMode); err != nil {
			return err
		}
	}
	return os.Chmod(root, dirMode)
}

func copyRestoreFile(sourcePath, targetPath string, mode os.FileMode) (retErr error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open temporary entry: %w", err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close temporary entry: %w", err))
		}
	}()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create staged file: %w", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		retErr = errors.Join(retErr, fmt.Errorf("copy temporary entry: %w", err))
	}
	if err := target.Sync(); err != nil {
		retErr = errors.Join(retErr, fmt.Errorf("sync staged file: %w", err))
	}
	if err := target.Close(); err != nil {
		retErr = errors.Join(retErr, fmt.Errorf("close staged file: %w", err))
	}
	if retErr != nil {
		return retErr
	}
	if err := os.Chmod(targetPath, mode); err != nil {
		return fmt.Errorf("set staged file permissions: %w", err)
	}
	return nil
}

func validateRestoreRoots(roots map[string]string) error {
	for _, name := range []string{"config", "data", "ssl"} {
		if roots[name] == "" {
			return fmt.Errorf("restore root %q is empty", name)
		}
	}
	for _, pair := range [][2]string{{"config", "data"}, {"data", "ssl"}} {
		if pathWithin(roots[pair[0]], roots[pair[1]]) || pathWithin(roots[pair[1]], roots[pair[0]]) {
			return fmt.Errorf("restore roots %q and %q overlap", pair[0], pair[1])
		}
	}
	if sameRestorePath(roots["config"], roots["ssl"]) {
		return fmt.Errorf("restore roots %q and %q must be distinct", "config", "ssl")
	}
	return nil
}

func sameRestorePath(first, second string) bool {
	firstAbs, err := filepath.Abs(filepath.Clean(first))
	if err != nil {
		return false
	}
	secondAbs, err := filepath.Abs(filepath.Clean(second))
	if err != nil {
		return false
	}
	return firstAbs == secondAbs
}

func pathWithin(parent, child string) bool {
	parentAbs, err := filepath.Abs(filepath.Clean(parent))
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(filepath.Clean(child))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
