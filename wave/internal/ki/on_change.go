package ki

import (
	"fmt"
	"strings"

	"github.com/vormadev/vorma/kit/executil"
	"golang.org/x/sync/errgroup"
)

const (
	OnChangeStrategyPre              = "pre"
	OnChangeStrategyPost             = "post"
	OnChangeStrategyConcurrent       = "concurrent"
	OnChangeStrategyConcurrentNoWait = "concurrent-no-wait"
)

type sortedOnChangeCallbacks struct {
	stratPre              []OnChangeHook
	stratConcurrent       []OnChangeHook
	stratPost             []OnChangeHook
	stratConcurrentNoWait []OnChangeHook
	exists                bool
}

func sortOnChangeCallbacks(onChanges []OnChangeHook) sortedOnChangeCallbacks {
	stratPre := []OnChangeHook{}
	stratConcurrent := []OnChangeHook{}
	stratPost := []OnChangeHook{}
	stratConcurrentNoWait := []OnChangeHook{}
	exists := false
	if len(onChanges) == 0 {
		return sortedOnChangeCallbacks{}
	} else {
		exists = true
	}
	for _, o := range onChanges {
		switch o.Timing {
		case OnChangeStrategyPre, "":
			stratPre = append(stratPre, o)
		case OnChangeStrategyConcurrent:
			stratConcurrent = append(stratConcurrent, o)
		case OnChangeStrategyPost:
			stratPost = append(stratPost, o)
		case OnChangeStrategyConcurrentNoWait:
			stratConcurrentNoWait = append(stratConcurrentNoWait, o)
		}
	}
	return sortedOnChangeCallbacks{
		stratPre:              stratPre,
		stratConcurrent:       stratConcurrent,
		stratPost:             stratPost,
		stratConcurrentNoWait: stratConcurrentNoWait,
		exists:                exists,
	}
}

func (c *Config) resolveCmd(cmd string) string {
	if cmd == "DevBuildHook" {
		return c._uc.Core.DevBuildHook
	}
	return cmd
}

func (c *Config) runConcurrentOnChangeCallbacks(onChanges []OnChangeHook, evtName string, shouldWait bool) error {
	if len(onChanges) > 0 {
		eg := errgroup.Group{}
		for _, o := range onChanges {
			if c.get_is_ignored(evtName, o.Exclude) {
				continue
			}
			eg.Go(func() error {
				err := executil.RunCmd(strings.Fields(c.resolveCmd(o.Cmd))...)
				if err != nil {
					c.Logger.Error(fmt.Sprintf("error running on-change callback: %v", err))
					return err
				}
				return nil
			})
		}

		if shouldWait {
			err := eg.Wait()
			if err != nil {
				return err
			}
		}

		// Wave has no control over error handling for "no-wait" callbacks
		// We simple call the function in a goroutine
		return nil
	}

	return nil
}

func (c *Config) simpleRunOnChangeCallbacks(onChanges []OnChangeHook, evtName string) error {
	for _, o := range onChanges {
		if c.get_is_ignored(evtName, o.Exclude) {
			continue
		}
		err := executil.RunCmd(strings.Fields(c.resolveCmd(o.Cmd))...)
		if err != nil {
			c.Logger.Error(fmt.Sprintf("error running on-change callback: %v", err))
			return err
		}
	}
	return nil
}
