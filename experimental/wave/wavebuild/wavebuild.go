package wavebuild

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
	"wave/internal/pkg/config"
	"wave/internal/pkg/cssbundle"
	"wave/internal/pkg/fswatcher"
	"wave/internal/pkg/staticproc"
	"wave/internal/pkg/workset"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/kit/strict"
)

/////////////////////////////////////////////////////////////////////
/////// Helpers
/////////////////////////////////////////////////////////////////////

func run_parallel(fns ...func() error) error {
	var mu sync.Mutex
	var first_err error
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				if first_err == nil {
					first_err = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return first_err
}

func path_match(
	pattern strict.CWDRelPath,
	path strict.CWDRelPath,
) (bool, error) {
	match, err := doublestar.PathMatch(string(pattern), string(path))
	if err != nil {
		return false, fmt.Errorf(
			"invalid pattern: %w", err,
		)
	}
	return match, nil
}

func extension(path strict.CWDRelPath) string {
	return filepath.Ext(string(path))
}

/////////////////////////////////////////////////////////////////////
/////// super_state
/////////////////////////////////////////////////////////////////////

type super_state struct {
	// config
	cfg_path strict.CWDRelPath
	cfg      config.Config
	cfg_hash string
	is_dev   bool

	// build state
	private_fm                *staticproc.Filemap
	public_fm                 *staticproc.Filemap
	critical_css_patterns     *set.Set[strict.CWDRelPath]
	non_critical_css_patterns *set.Set[strict.CWDRelPath]
	watcher                   *fswatcher.Watcher
	logger                    *slog.Logger

	// watch loop state (guarded by mu)
	mu                sync.Mutex
	pending_triggers  *set.Set[workset.Trigger]
	pending_evt_paths *set.Set[strict.CWDRelPath]
	is_building       bool
}

func hash_cfg(cfg config.Config) string {
	json, err := jsonutil.Serialize(cfg.Get())
	if err != nil {
		panic("failed to serialize cfg: " + err.Error())
	}
	return bytesutil.ToBase64(cryptoutil.Sha256Hash(json))
}

func (s *super_state) parse_cfg() error {
	s.logger.Info("parsing config...")
	old_root := strict.CWDRelPath("")
	if s.cfg != nil {
		old_root = s.cfg.Get().RootDir
	}
	cfg, err := config.ConfigPathToValidatedConfig(s.cfg_path)
	if err != nil {
		return fmt.Errorf("config parse failed: %w", err)
	}
	s.cfg = cfg
	s.cfg_hash = hash_cfg(s.cfg)
	if s.watcher != nil && s.cfg.Get().RootDir != old_root {
		s.watcher.SetWatchRoot(s.cfg.Get().RootDir)
	}
	return nil
}

func (s *super_state) derive_opts(
	triggers *set.Set[workset.Trigger],
) workset.DeriveOpts {
	cfg := s.cfg.Get()
	return workset.DeriveOpts{
		Triggers:          triggers,
		HasPrivateStatic:  cfg.HasPrivateStatic(),
		HasPublicStatic:   cfg.HasPublicStatic(),
		HasCriticalCSS:    cfg.HasCriticalCSS(),
		HasNonCriticalCSS: cfg.HasNonCriticalCSS(),
		UsingVite:         cfg.Vite.UsingVite,
	}
}

func (s *super_state) log_build_err(err error) {
	s.logger.Warn(fmt.Sprintf(
		"build error — %s — edit the offending watched files to retry",
		err,
	))
}

/////////////////////////////////////////////////////////////////////
/////// Event classification
/////////////////////////////////////////////////////////////////////

type classify_result struct {
	trigger workset.Trigger
	matched bool
}

func (s *super_state) classify_evt(evt fswatcher.Evt) (classify_result, error) {
	cfg := s.cfg.Get()

	// global excludes
	for _, pattern := range cfg.Core.GlobalWatchExcludePatterns {
		match, err := path_match(pattern, evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid global watch exclude pattern: %w", err,
			)
		}
		if match {
			return classify_result{matched: false}, nil
		}
	}

	// private static
	if cfg.HasPrivateStatic() {
		priv_match, err := path_match(cfg.PrivateDirPattern(), evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid private pattern: %w",
				err,
			)
		}
		if priv_match {
			s.logger.Info("private asset changed")
			return classify_result{
				trigger: workset.PrivateStaticSrcChanged, matched: true,
			}, nil
		}
	}

	// public static
	if cfg.HasPublicStatic() {
		pub_match, err := path_match(cfg.PublicDirPattern(), evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid public pattern: %w",
				err,
			)
		}
		if pub_match {
			s.logger.Info("public asset changed")
			return classify_result{
				trigger: workset.PublicStaticSrcChanged, matched: true,
			}, nil
		}
	}

	// critical css
	has_css_ext := extension(evt.Path) == ".css"
	if has_css_ext &&
		s.critical_css_patterns != nil &&
		s.critical_css_patterns.Has(evt.Path) {
		s.logger.Info("critical css changed")
		return classify_result{
			trigger: workset.CriticalCSSSrcChanged, matched: true,
		}, nil
	}

	// non-critical css
	if has_css_ext &&
		s.non_critical_css_patterns != nil &&
		s.non_critical_css_patterns.Has(evt.Path) {
		s.logger.Info("non-critical css changed")
		return classify_result{
			trigger: workset.NonCriticalCSSSrcChanged, matched: true,
		}, nil
	}

	// implicit go
	if extension(evt.Path) == ".go" {
		is_go, err := path_match(cfg.GoFilePattern(), evt.Path)
		if err != nil {
			return classify_result{}, fmt.Errorf(
				"invalid global go pattern: %w", err,
			)
		}
		if is_go {
			for _, pattern := range cfg.Core.PreventImplicitGoBuildPatterns {
				match, err := path_match(pattern, evt.Path)
				if err != nil {
					return classify_result{}, fmt.Errorf(
						"invalid implicit go build exclusion pattern: %w", err,
					)
				}
				if match {
					return classify_result{matched: false}, nil
				}
			}
			s.logger.Info("go file changed", "path", evt.Path)
			return classify_result{
				trigger: workset.GoSrcChanged, matched: true,
			}, nil
		}
	}

	return classify_result{matched: false}, nil
}

/////////////////////////////////////////////////////////////////////
/////// Build phases
/////////////////////////////////////////////////////////////////////

func (s *super_state) get_private_filemap(
	evt_paths *set.Set[strict.CWDRelPath],
) error {
	if !s.cfg.Get().HasPrivateStatic() {
		return nil
	}
	s.logger.Info("getting private filemap...")
	var err error
	sp := &staticproc.StaticProcessor{
		SrcDir: s.cfg.Get().Core.StaticAssetDirs.Private,
		OutDir: s.cfg_path.Dir().Join(".waveout/static/assets/private"),
	}
	s.private_fm, err = sp.PhysicalFilemap(s.private_fm, evt_paths)
	if err != nil {
		return fmt.Errorf("getting private filemap failed: %w", err)
	}
	return nil
}

func (s *super_state) get_public_filemap(
	evt_paths *set.Set[strict.CWDRelPath],
) error {
	if !s.cfg.Get().HasPublicStatic() {
		return nil
	}
	s.logger.Info("getting public filemap...")
	var err error
	sp := &staticproc.StaticProcessor{
		SrcDir:              s.cfg.Get().Core.StaticAssetDirs.Public,
		PassthroughDirnames: []string{"__nohash", "prehashed"},
		OutDir: s.cfg_path.Dir().
			Join(".waveout/static/assets/public"),
		OutFilePrefix: "wave_out_",
	}
	s.public_fm, err = sp.PhysicalFilemap(s.public_fm, evt_paths)
	if err != nil {
		return fmt.Errorf("getting public filemap failed: %w", err)
	}
	return nil
}

func (s *super_state) build_critical_css() error {
	if !s.cfg.Get().HasCriticalCSS() {
		return nil
	}
	s.logger.Info("building critical css...")
	css_bundle_out, err := cssbundle.Bundle(
		s.cfg.Get().Core.CSSEntryFiles.Critical,
		s.public_fm.Map(),
	)
	if err != nil {
		return fmt.Errorf("critical css bundle failed: %w", err)
	}
	s.critical_css_patterns = set.New(css_bundle_out.Imports)
	critical_css_out_path := string(
		s.cfg_path.Dir().Join(".waveout/static/internal/critical.css"),
	)
	if err := os.WriteFile(critical_css_out_path, []byte(css_bundle_out.CSS), 0644); err != nil {
		return fmt.Errorf("failed to write critical css: %w", err)
	}
	return nil
}

func (s *super_state) build_non_critical_css() error {
	if !s.cfg.Get().HasNonCriticalCSS() {
		return nil
	}
	s.logger.Info("building non-critical css...")
	css_bundle_out, err := cssbundle.Bundle(
		s.cfg.Get().Core.CSSEntryFiles.NonCritical,
		s.public_fm.Map(),
	)
	if err != nil {
		return fmt.Errorf("non-critical css bundle failed: %w", err)
	}
	s.non_critical_css_patterns = set.New(css_bundle_out.Imports)
	s.public_fm.Set(map[string][]byte{
		"__wave_owned_non_critical.css": []byte(css_bundle_out.CSS),
	})
	return nil
}

func (s *super_state) apply_diff(
	fm *staticproc.Filemap, json_path strict.CWDRelPath,
) error {
	s.logger.Info(fmt.Sprintf(
		"applying static diff (%s)...",
		filepath.Base(string(fm.OutDir())),
	))
	return fm.ApplyDiffAndWriteFilemapJSON(json_path)
}

// run_build returns (completed, error).
// completed=false means aborted early because new events are pending.
func (s *super_state) run_build(
	fx *set.Set[workset.Effect],
	evt_paths *set.Set[strict.CWDRelPath],
	should_abort func() bool,
	is_initial_build bool,
) (bool, error) {
	s.logger.Info("running build...")

	if s.is_dev && !is_initial_build && fx.Has(workset.ShowRebuildingOverlay) {
		s.logger.Info("triggering frontend rebuilding overlay...")
	}

	// phase 1: get static filemaps in parallel
	var filemap_fns []func() error
	if fx.Has(workset.BuildPrivateFilemap) {
		filemap_fns = append(filemap_fns, func() error {
			return s.get_private_filemap(evt_paths)
		})
	}
	if fx.Has(workset.BuildPublicFilemap) {
		filemap_fns = append(filemap_fns, func() error {
			return s.get_public_filemap(evt_paths)
		})
	}
	if err := run_parallel(filemap_fns...); err != nil {
		return true, err
	}

	// ensure public_fm exists before css phase (may be phantom if no
	// public dir is configured but css entries are set)
	if s.public_fm == nil &&
		(fx.Has(workset.BuildCriticalCSS) || fx.Has(workset.BuildNonCriticalCSS)) {
		sp := &staticproc.StaticProcessor{
			OutDir: s.cfg_path.Dir().
				Join(".waveout/static/assets/public"),
			OutFilePrefix: "wave_out_",
		}
		s.public_fm = sp.PhantomFilemap()
	}

	// phase 2: css bundles in parallel
	// (must follow public filemap build because css reads from public_fm)
	var css_fns []func() error
	if fx.Has(workset.BuildCriticalCSS) {
		css_fns = append(css_fns, s.build_critical_css)
	}
	if fx.Has(workset.BuildNonCriticalCSS) {
		css_fns = append(css_fns, s.build_non_critical_css)
	}
	if err := run_parallel(css_fns...); err != nil {
		return true, err
	}

	// phase 3: apply diffs in parallel
	var diff_fns []func() error
	if fx.Has(workset.BuildPrivateFilemap) && s.private_fm != nil {
		diff_fns = append(diff_fns, func() error {
			return s.apply_diff(
				s.private_fm,
				s.cfg_path.Dir().Join(
					".waveout/static/internal/private_filemap.json",
				),
			)
		})
	}
	if s.public_fm != nil &&
		(fx.Has(workset.BuildPublicFilemap) ||
			fx.Has(workset.BuildCriticalCSS) ||
			fx.Has(workset.BuildNonCriticalCSS)) {
		diff_fns = append(diff_fns, func() error {
			return s.apply_diff(
				s.public_fm,
				s.cfg_path.Dir().Join(
					".waveout/static/internal/public_filemap.json",
				),
			)
		})
	}
	if err := run_parallel(diff_fns...); err != nil {
		return true, err
	}

	// phase 4: sequential steps with abort checks

	if fx.Has(workset.CompileGo) {
		s.logger.Info("compiling go...")
		if should_abort() {
			s.logger.Info(
				"aborting before go compile (new events pending)",
			)
			return false, nil
		}
		time.Sleep(1 * time.Second)
	}

	if s.is_dev && !is_initial_build {
		if fx.Has(workset.RestartAppServer) {
			if should_abort() {
				s.logger.Info(
					"aborting before app restart (new events pending)",
				)
				return false, nil
			}
			time.Sleep(1 * time.Second)
		}

		if fx.Has(workset.HardReloadBrowser) ||
			fx.Has(workset.ClientSideRevalidate) ||
			fx.Has(workset.HotCSSRefresh) {
			if should_abort() {
				s.logger.Info(
					"aborting before frontend settling (new events pending)",
				)
				return false, nil
			}
			s.logger.Info("triggering frontend settling",
				"hard_reload", fx.Has(workset.HardReloadBrowser),
				"hot_css_refresh", fx.Has(workset.HotCSSRefresh),
				"hot_revalidate", fx.Has(workset.ClientSideRevalidate),
			)
		}
	}

	return true, nil
}

/////////////////////////////////////////////////////////////////////
/////// Watch loop
/////////////////////////////////////////////////////////////////////

func (s *super_state) should_abort() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending_triggers != nil
}

func (s *super_state) drain_and_build() {
	for {
		s.mu.Lock()
		if s.pending_triggers == nil {
			s.is_building = false
			s.mu.Unlock()
			return
		}
		triggers := s.pending_triggers
		evt_paths := s.pending_evt_paths
		s.pending_triggers = nil
		s.pending_evt_paths = nil
		s.mu.Unlock()

		// full rebuild doesn't use incremental paths
		if triggers.Has(workset.ConfigChanged) {
			evt_paths = nil
		}

		fx := workset.Derive(s.derive_opts(triggers))

		completed, err := s.run_build(fx, evt_paths, s.should_abort, false)
		if err != nil {
			s.log_build_err(err)
			s.mu.Lock()
			s.is_building = false
			s.mu.Unlock()
			return
		}
		if !completed {
			// aborted early — merge the original needs back so the
			// expensive steps are retried on the next iteration
			s.mu.Lock()
			if s.pending_triggers == nil {
				s.pending_triggers = triggers
			} else {
				s.pending_triggers.Union(triggers)
			}
			if evt_paths != nil {
				if s.pending_evt_paths == nil {
					s.pending_evt_paths = evt_paths
				} else {
					s.pending_evt_paths.Union(evt_paths)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *super_state) handle_watch_evts(evts []fswatcher.Evt) error {
	// If a ConfigChanged full rebuild is already pending, skip
	// classification — just ensure drain_and_build is running.
	s.mu.Lock()
	if s.pending_triggers != nil &&
		s.pending_triggers.Has(workset.ConfigChanged) {
		if !s.is_building {
			s.is_building = true
			go s.drain_and_build()
		}
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// classify events into triggers
	triggers := &set.Set[workset.Trigger]{}
	var parse_err error
	for _, evt := range evts {
		// config gets special handling: parse eagerly and check hash
		if evt.Path == s.cfg_path {
			old_cfg_hash := s.cfg_hash
			if err := s.parse_cfg(); err != nil {
				parse_err = err
				break
			}
			if s.cfg_hash != old_cfg_hash {
				s.logger.Info(
					"config changed, triggering full build...",
				)
				triggers.Add(workset.ConfigChanged)
				break
			}
			s.logger.Info("config file unchanged")
			continue
		}

		result, err := s.classify_evt(evt)
		if err != nil {
			parse_err = err
			break
		}
		if result.matched {
			triggers.Add(result.trigger)
		}
	}

	evt_paths := &set.Set[strict.CWDRelPath]{}
	for _, evt := range evts {
		evt_paths.Add(evt.Path)
	}

	s.mu.Lock()
	if parse_err != nil {
		// Log and do nothing. The config or patterns are broken; the
		// user must fix the offending file. The next FS event on that
		// file will retry naturally.
		s.log_build_err(parse_err)
	} else if triggers.Len() > 0 {
		if s.pending_triggers == nil {
			s.pending_triggers = triggers
		} else {
			s.pending_triggers.Union(triggers)
		}
		if s.pending_evt_paths == nil {
			s.pending_evt_paths = evt_paths
		} else {
			s.pending_evt_paths.Union(evt_paths)
		}
	}
	if s.pending_triggers != nil && !s.is_building {
		s.is_building = true
		go s.drain_and_build()
	}
	s.mu.Unlock()
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// Entrypoint
/////////////////////////////////////////////////////////////////////

type BuildOpts struct {
	ConfigPath strict.CWDRelPath
	IsDev      bool
	Logger     *slog.Logger
}

func Build(opts BuildOpts) {
	s := &super_state{
		cfg_path: strict.MustNormalize(opts.ConfigPath),
		is_dev:   opts.IsDev,
		logger:   colorlog.New("wave"),
	}

	/////// initial build

	if err := s.parse_cfg(); err != nil {
		s.logger.Error(err.Error())
		os.Exit(1)
	}

	fsutil.EnsureDir(string(s.cfg_path.Dir().Join(".waveout/static/internal")))

	json_schema, err := config.BuildSchema(config.BuildSchemaOpts{})
	if err != nil {
		s.logger.Error("failed to build config schema: " + err.Error())
		os.Exit(1)
	}
	json_schema_bytes, err := jsonutil.SerializePretty(json_schema)
	if err != nil {
		s.logger.Error("failed to serialize config schema: " + err.Error())
		os.Exit(1)
	}
	if err := os.WriteFile(
		string(s.cfg_path.Dir().Join(".waveout/schema.json")),
		append(json_schema_bytes, '\n'),
		0644,
	); err != nil {
		s.logger.Error("failed to write config schema: " + err.Error())
		os.Exit(1)
	}

	initial_triggers := &set.Set[workset.Trigger]{}
	initial_triggers.Add(workset.ConfigChanged)
	fx := workset.Derive(s.derive_opts(initial_triggers))
	if _, err := s.run_build(fx, nil, func() bool { return false }, true); err != nil {
		s.logger.Error("initial build failed: " + err.Error())
		os.Exit(1)
	}

	/////// watch loop

	if s.is_dev {
		s.watcher = fswatcher.NewWatcher(fswatcher.WatcherOptions{
			WatchRoot: s.cfg.Get().RootDir,
			OnRemovePath: func(p strict.CWDRelPath) {
				s.logger.Info("removing watch on", "path", p)
			},
		})

		s.watcher.Watch(context.Background(), func(evts []fswatcher.Evt) error {
			return s.handle_watch_evts(evts)
		})
	}
}
