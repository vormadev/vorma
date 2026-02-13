package tooling

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

type watcherEventPostClassificationDecision struct {
	includeClassifiedEvent bool
}

func shouldLogWatcherAddDirectoryError(
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

func deriveWatcherEventPostClassificationDecision(
	classifiedEventForProcessing classifiedEvent,
) watcherEventPostClassificationDecision {
	if classifiedEventForProcessing.ignored || classifiedEventForProcessing.chmodOnly {
		return watcherEventPostClassificationDecision{}
	}

	return watcherEventPostClassificationDecision{
		includeClassifiedEvent: true,
	}
}

func filterClassifiedEventsForProcessingByPostClassificationDecision(
	classifiedEvents []classifiedEvent,
) []classifiedEvent {
	if len(classifiedEvents) == 0 {
		return nil
	}

	filteredClassifiedEvents := make([]classifiedEvent, 0, len(classifiedEvents))
	for _, classifiedEventForProcessing := range classifiedEvents {
		postClassificationDecision := deriveWatcherEventPostClassificationDecision(
			classifiedEventForProcessing,
		)
		if !postClassificationDecision.includeClassifiedEvent {
			continue
		}
		filteredClassifiedEvents = append(
			filteredClassifiedEvents,
			classifiedEventForProcessing,
		)
	}
	return filteredClassifiedEvents
}
