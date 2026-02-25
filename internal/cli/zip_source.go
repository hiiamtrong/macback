package cli

import (
	"fmt"
	"strings"

	"github.com/hiiamtrong/macback/internal/fsutil"
)

// noop is a no-op cleanup function used when no temporary resources were created.
func noop() { /* no temporary resources to clean up */ }

// resolveBackupSource expands path and, if it is a .zip file, extracts it to a
// temporary directory. Returns the directory to use as the backup source and a
// cleanup function that must be called when done (safe to call even on error).
func resolveBackupSource(source string) (dir string, cleanup func(), err error) {
	expanded, err := fsutil.ExpandPath(source)
	if err != nil {
		return "", noop, fmt.Errorf("expanding source path: %w", err)
	}

	if strings.HasSuffix(expanded, ".zip") {
		dir, cleanup, err = fsutil.UnzipToTemp(expanded)
		if err != nil {
			return "", noop, fmt.Errorf("extracting zip: %w", err)
		}
		return dir, cleanup, nil
	}

	return expanded, noop, nil
}
