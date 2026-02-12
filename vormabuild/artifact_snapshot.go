package vormabuild

import (
	"errors"
	"os"
	"syscall"
)

type buildArtifactFileSnapshot struct {
	existed bool
	content []byte
}

func captureBuildArtifactFileSnapshot(
	artifactPath string,
	readArtifactFile func(string) ([]byte, error),
) (buildArtifactFileSnapshot, error) {
	artifactContent, err := readArtifactFile(artifactPath)
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
			return buildArtifactFileSnapshot{}, nil
		}
		return buildArtifactFileSnapshot{}, err
	}

	return buildArtifactFileSnapshot{existed: true, content: artifactContent}, nil
}

func restoreBuildArtifactFileSnapshot(
	artifactPath string,
	snapshot buildArtifactFileSnapshot,
	writeArtifactFile func(string, []byte, os.FileMode) error,
	removeArtifactFile func(string) error,
) error {
	if snapshot.existed {
		return writeArtifactFile(artifactPath, snapshot.content, buildArtifactFileMode)
	}

	err := removeArtifactFile(artifactPath)
	if err == nil || os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}
