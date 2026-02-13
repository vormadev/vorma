package tooling

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/wave"
)

func BuildWaveExplainReport(cfg *wave.ParsedConfig) string {
	var reportBuilder strings.Builder

	writeReportLine(&reportBuilder, "Wave Explain")
	writeReportLine(&reportBuilder, "===========")

	if cfg == nil {
		writeReportLine(&reportBuilder, "config: <nil>")
		return reportBuilder.String()
	}

	writeReportLine(&reportBuilder, fmt.Sprintf("using_browser: %t", cfg.UsingBrowser()))
	writeReportLine(&reportBuilder, fmt.Sprintf("using_vite: %t", cfg.UsingVite()))

	if cfg.Core != nil {
		writeReportLine(&reportBuilder, fmt.Sprintf("main_app_entry: %s", cfg.Core.MainAppEntry))
		writeReportLine(&reportBuilder, fmt.Sprintf("dist_root: %s", cfg.Dist.Root))
		writeReportLine(&reportBuilder, fmt.Sprintf("dist_binary: %s", cfg.Dist.Binary()))
		writeReportLine(&reportBuilder, fmt.Sprintf("public_static_dir: %s", cfg.Core.StaticAssetDirs.Public))
		writeReportLine(&reportBuilder, fmt.Sprintf("private_static_dir: %s", cfg.Core.StaticAssetDirs.Private))
	}

	writeReportLine(&reportBuilder, fmt.Sprintf("watch_root: %s", cfg.WatchRoot()))
	writeReportLine(&reportBuilder, fmt.Sprintf("healthcheck_endpoint: %s", cfg.HealthcheckEndpoint()))
	writeReportLine(&reportBuilder, fmt.Sprintf("watch_include_patterns: %d", len(getUserWatchPatterns(cfg))))
	writeReportLine(&reportBuilder, fmt.Sprintf("framework_watch_patterns: %d", len(cfg.FrameworkWatchPatterns)))
	writeReportLine(&reportBuilder, fmt.Sprintf("framework_ignored_patterns: %d", len(cfg.FrameworkIgnoredPatterns)))
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("framework_public_filemap_out_dir: %s", cfg.FrameworkPublicFileMapOutDir),
	)

	userHookCommand := getUserDevBuildHook(cfg)
	frameworkHookCommand := getFrameworkDevBuildHook(cfg)
	writeReportLine(&reportBuilder, fmt.Sprintf("core_dev_build_hook: %s", emptyAsNone(userHookCommand)))
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("framework_dev_build_hook: %s", emptyAsNone(frameworkHookCommand)),
	)

	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("config_file_path: %s", emptyAsNone(cfg.GetResolvedConfigFilePath())),
	)
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("config_fingerprint: %s", emptyAsNone(cfg.GetResolvedConfigFingerprint())),
	)

	hookSummary := collectHookSummary(cfg)
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("hooks_total: %d", hookSummary.totalHooks),
	)
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("hooks_cmd: %d", hookSummary.commandHooks),
	)
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("hooks_run_combined_dev_build: %d", hookSummary.combinedDevBuildHooks),
	)
	writeReportLine(
		&reportBuilder,
		fmt.Sprintf("hooks_callback: %d", hookSummary.callbackHooks),
	)

	return reportBuilder.String()
}

func BuildWaveDoctorReport(cfg *wave.ParsedConfig) (string, bool) {
	var issues []string
	var notes []string

	if cfg == nil {
		issues = append(issues, "config is nil")
	} else {
		if configValidationErr := ValidateConfig(cfg); configValidationErr != nil {
			issues = append(
				issues,
				fmt.Sprintf("config validation failed: %v", configValidationErr),
			)
		}

		watchRoot := cfg.WatchRoot()
		if _, err := os.Stat(watchRoot); err != nil {
			issues = append(issues, fmt.Sprintf("watch root is not accessible: %s (%v)", watchRoot, err))
		}

		notes = append(notes, collectDuplicatePatternNotes(cfg)...)
		issues = append(issues, collectHookConfigurationIssues(cfg)...)
		issues = append(issues, collectWatchPatternIssues(cfg)...)
		configFileIssues, configFileNotes := collectConfigFileHealth(cfg)
		issues = append(issues, configFileIssues...)
		notes = append(notes, configFileNotes...)
	}

	var reportBuilder strings.Builder
	writeReportLine(&reportBuilder, "Wave Doctor")
	writeReportLine(&reportBuilder, "===========")
	if len(issues) == 0 {
		writeReportLine(&reportBuilder, "issues: none")
	} else {
		writeReportLine(&reportBuilder, "issues:")
		for _, issue := range issues {
			writeReportLine(&reportBuilder, fmt.Sprintf("- %s", issue))
		}
	}
	if len(notes) > 0 {
		writeReportLine(&reportBuilder, "notes:")
		for _, note := range notes {
			writeReportLine(&reportBuilder, fmt.Sprintf("- %s", note))
		}
	}

	return reportBuilder.String(), len(issues) > 0
}

type hookSummary struct {
	totalHooks            int
	commandHooks          int
	combinedDevBuildHooks int
	callbackHooks         int
}

func collectHookSummary(cfg *wave.ParsedConfig) hookSummary {
	var summary hookSummary

	watchedFiles := collectAllWatchedFiles(cfg)
	for _, watchedFile := range watchedFiles {
		for _, hook := range watchedFile.OnChangeHooks {
			summary.totalHooks++
			if strings.TrimSpace(hook.Cmd) != "" {
				summary.commandHooks++
			}
			if hook.RunCombinedDevBuildHookCommands {
				summary.combinedDevBuildHooks++
			}
			if hook.Callback != nil {
				summary.callbackHooks++
			}
		}
	}

	return summary
}

func collectDuplicatePatternNotes(cfg *wave.ParsedConfig) []string {
	duplicatePatternCountByPattern := map[string]int{}
	for _, watchedFile := range collectAllWatchedFiles(cfg) {
		trimmedPattern := strings.TrimSpace(watchedFile.Pattern)
		if trimmedPattern == "" {
			continue
		}
		duplicatePatternCountByPattern[trimmedPattern]++
	}

	var duplicatePatterns []string
	for pattern, count := range duplicatePatternCountByPattern {
		if count < 2 {
			continue
		}
		duplicatePatterns = append(
			duplicatePatterns,
			fmt.Sprintf("duplicate watch pattern %q appears %d times", pattern, count),
		)
	}
	slices.Sort(duplicatePatterns)
	return duplicatePatterns
}

func collectHookConfigurationIssues(cfg *wave.ParsedConfig) []string {
	var issues []string

	userHookCommand := strings.TrimSpace(getUserDevBuildHook(cfg))
	frameworkHookCommand := strings.TrimSpace(getFrameworkDevBuildHook(cfg))

	for watchedFileIndex, watchedFile := range collectAllWatchedFiles(cfg) {
		if watchedFile.RunOnChangeOnly && len(watchedFile.OnChangeHooks) == 0 {
			issues = append(
				issues,
				fmt.Sprintf(
					"watch_pattern[%d=%q] sets RunOnChangeOnly but defines no OnChangeHooks",
					watchedFileIndex,
					watchedFile.Pattern,
				),
			)
		}

		for hookIndex, hook := range watchedFile.OnChangeHooks {
			hookPath := fmt.Sprintf(
				"watch_pattern[%d=%q].on_change_hooks[%d]",
				watchedFileIndex,
				watchedFile.Pattern,
				hookIndex,
			)

			hasCommand := strings.TrimSpace(hook.Cmd) != ""
			hasCallback := hook.Callback != nil
			hasCommandLikeAction := hasCommand || hook.RunCombinedDevBuildHookCommands

			if !hasCommandLikeAction && !hasCallback {
				issues = append(
					issues,
					fmt.Sprintf(
						"%s has no executable action (Cmd, RunCombinedDevBuildHookCommands, or Callback)",
						hookPath,
					),
				)
			}

			if hasCommand && hook.RunCombinedDevBuildHookCommands {
				issues = append(
					issues,
					fmt.Sprintf(
						"%s sets both Cmd and RunCombinedDevBuildHookCommands",
						hookPath,
					),
				)
			}

			if watchedFile.RunOnChangeOnly &&
				hasCommandLikeAction &&
				hook.Timing != "" &&
				hook.Timing != wave.OnChangeStrategyPre {
				issues = append(
					issues,
					fmt.Sprintf(
						"%s uses timing %q, but RunOnChangeOnly command hooks must use pre timing",
						hookPath,
						hook.Timing,
					),
				)
			}

			if hook.RunCombinedDevBuildHookCommands &&
				userHookCommand == "" &&
				frameworkHookCommand == "" &&
				!hasCommand {
				issues = append(
					issues,
					fmt.Sprintf(
						"%s enables RunCombinedDevBuildHookCommands but no dev build hooks are configured",
						hookPath,
					),
				)
			}
		}
	}

	slices.Sort(issues)
	return issues
}

func collectWatchPatternIssues(cfg *wave.ParsedConfig) []string {
	var issues []string

	for watchedFileIndex, watchedFile := range collectAllWatchedFiles(cfg) {
		trimmedPattern := strings.TrimSpace(watchedFile.Pattern)
		if trimmedPattern == "" {
			issues = append(
				issues,
				fmt.Sprintf("watch_pattern[%d] has empty Pattern", watchedFileIndex),
			)
			continue
		}

		if watchPatternContainsGlobMeta(trimmedPattern) && !doublestar.ValidatePattern(trimmedPattern) {
			issues = append(
				issues,
				fmt.Sprintf(
					"watch_pattern[%d=%q] contains an invalid glob pattern",
					watchedFileIndex,
					watchedFile.Pattern,
				),
			)
		}
	}

	slices.Sort(issues)
	return issues
}

func collectConfigFileHealth(cfg *wave.ParsedConfig) ([]string, []string) {
	var issues []string
	var notes []string

	if cfg == nil {
		return issues, notes
	}

	resolvedConfigFilePath := strings.TrimSpace(cfg.GetResolvedConfigFilePath())
	if resolvedConfigFilePath == "" {
		notes = append(
			notes,
			"resolved config file path is unset; config edits will not trigger config reload",
		)
		slices.Sort(notes)
		return issues, notes
	}

	if _, err := os.Stat(resolvedConfigFilePath); err != nil {
		issues = append(
			issues,
			fmt.Sprintf(
				"resolved config file path %q is not accessible (%v)",
				resolvedConfigFilePath,
				err,
			),
		)
	}

	slices.Sort(issues)
	slices.Sort(notes)
	return issues, notes
}

func watchPatternContainsGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

func collectAllWatchedFiles(cfg *wave.ParsedConfig) []wave.WatchedFile {
	if cfg == nil {
		return nil
	}

	allWatchedFiles := make([]wave.WatchedFile, 0, len(getUserWatchPatterns(cfg))+len(cfg.FrameworkWatchPatterns))
	allWatchedFiles = append(allWatchedFiles, getUserWatchPatterns(cfg)...)
	allWatchedFiles = append(allWatchedFiles, cfg.FrameworkWatchPatterns...)
	return allWatchedFiles
}

func getUserWatchPatterns(cfg *wave.ParsedConfig) []wave.WatchedFile {
	if cfg == nil || cfg.Watch == nil {
		return nil
	}
	return cfg.Watch.Include
}

func writeReportLine(reportBuilder *strings.Builder, line string) {
	reportBuilder.WriteString(line)
	reportBuilder.WriteString("\n")
}

func emptyAsNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<none>"
	}
	return value
}
