package watchereventclassification

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// PostClassificationDecision resolves whether a classified event should move
// forward into pipeline planning.
type PostClassificationDecision struct {
	IncludeClassifiedEvent bool
}

// DerivePostClassificationDecision resolves event inclusion after mapping.
func DerivePostClassificationDecision(
	classifiedEventIgnored bool,
	classifiedEventIsChmodOnly bool,
) PostClassificationDecision {
	if classifiedEventIgnored || classifiedEventIsChmodOnly {
		return PostClassificationDecision{}
	}

	return PostClassificationDecision{
		IncludeClassifiedEvent: true,
	}
}

// ShouldLogAddDirectoryWatchError suppresses expected watcher add-dir probe
// failures and preserves logging for actionable errors.
func ShouldLogAddDirectoryWatchError(
	addDirectoryWatchError error,
) bool {
	if addDirectoryWatchError == nil {
		return false
	}

	if os.IsNotExist(addDirectoryWatchError) || errors.Is(addDirectoryWatchError, fs.ErrNotExist) {
		return false
	}

	if errors.Is(addDirectoryWatchError, syscall.ENOTDIR) {
		return false
	}

	return true
}
