package scout

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// errNotGitRepo is returned by gitTrackedFiles when root is not inside a
// git working tree, or when git itself is not on PATH. Callers treat it
// as the signal to fall back to filesystem walking.
var errNotGitRepo = errors.New("scout: not a git repo")

// gitTrackedFiles returns the paths git would consider for this repo
// (tracked files plus untracked files not matched by .gitignore), each
// relative to root, slash-separated. The cost is two short subprocess
// calls per scan, which is cheap compared to the file reads we do
// afterward and saves us from reimplementing .gitignore parsing.
//
// Returns errNotGitRepo if root is not in a git working tree or git is
// missing. Returns a real error if git was found and ran but failed.
func gitTrackedFiles(ctx context.Context, root string) ([]string, error) {
	// Detection: rev-parse exits 0 inside a working tree. Stdout/stderr
	// default to /dev/null so this stays quiet outside a repo.
	if err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--git-dir").Run(); err != nil {
		return nil, errNotGitRepo
	}

	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files",
		"--cached", "--others", "--exclude-standard")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	var files []string
	sc := bufio.NewScanner(&stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			files = append(files, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read git output: %w", err)
	}
	return files, nil
}
