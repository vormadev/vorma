package vormabuild

import (
	"context"
	"fmt"
	"os"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/searchparams"
	"github.com/vormadev/vorma/kit/tsgen"
)

type live_state struct {
	Error         string
	VormaConfig   *vorma.Vorma
	TSResult      live_ts_result
	TSModules     map[string]ts_route
	SearchSchemas map[string]searchparams.Schema
}

/////////////////////////////////////////////////////////////////////
/////// LIVE STATE -- Command Runner
/////////////////////////////////////////////////////////////////////

// Runs build entry in gen mode.
// Causes build entry to print gen state as JSON, which is read and returned.
func (cfg vorma_cfg) read_live_state_from_subprocess(
	ctx context.Context,
	build_entry string,
) (*live_state, error) {
	cmd := new_cmd(ctx, "go", "run", build_entry)
	cmd.Env = append(cmd.Env, live_state_mode_env_key+"=1")
	o, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(
			"error running builder entry command 'go run %s': %w | output: %s",
			build_entry,
			err,
			string(o),
		)
	}
	ls, err := jsonutil.Parse[*live_state](o)
	if err != nil {
		return nil, fmt.Errorf("error parsing builder entry output: %w", err)
	}
	if ls.Error != "" {
		return nil, fmt.Errorf("error from builder entry: %s", ls.Error)
	}
	return ls, nil
}

func get_live_state(
	v *vorma.Vorma,
	loaders vorma.Loaders,
	actions vorma.Actions,
) (*live_state, error) {
	cfg, err := to_cfg(v)
	if err != nil {
		return nil, fmt.Errorf("error converting config: %w", err)
	}

	extra_ts_str := ""
	if v.TSGenConfig.ExtraRawTS != nil {
		d := v.TSGenConfig.ExtraRawTS(&tsgen.TSDrafter{})
		if d != nil {
			extra_ts_str = d.String()
		}
	}

	ts_result, err := cfg.to_live_ts_result(
		loaders,
		actions,
		v.TSGenConfig.ExtraTypes,
		extra_ts_str,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating TS types: %w", err)
	}

	ts_modules, err := cfg.get_dev_ts_modules(loaders)
	if err != nil {
		return nil, fmt.Errorf("error getting TS modules: %w", err)
	}

	search_schemas := make(map[string]searchparams.Schema, len(loaders))
	for _, l := range loaders {
		pattern := l.GetPattern()
		if pattern == "" {
			continue
		}
		schema, err := searchparams.SchemaFromValue(l.IType().Instance)
		if err != nil {
			return nil, fmt.Errorf(
				"error generating loader search schema for %s: %w",
				pattern,
				err,
			)
		}
		search_schemas[pattern] = schema
	}

	ls := &live_state{
		VormaConfig:   cfg.V,
		TSResult:      ts_result,
		TSModules:     ts_modules,
		SearchSchemas: search_schemas,
	}

	return ls, nil
}

func print_live_state_and_exit(
	v *vorma.Vorma,
	loaders vorma.Loaders,
	actions vorma.Actions,
) {
	ls, err := get_live_state(v, loaders, actions)
	if err != nil {
		print_err_json_and_exit("error getting live state", err)
	}

	gen_state_json, err := jsonutil.SerializePretty(ls)
	if err != nil {
		print_err_json_and_exit("error serializing gen state", err)
	}

	fmt.Print(string(gen_state_json))
	os.Exit(0)
}

func print_err_json_and_exit(outer_msg string, err error) {
	err_json, marshal_err := jsonutil.SerializePretty(map[string]string{
		"Error": fmt.Sprintf("%s: %s", outer_msg, err.Error()),
	})
	if marshal_err != nil {
		fmt.Println(`{"Error":"unknown error"}`)
		os.Exit(1)
	}
	fmt.Println(string(err_json))
	os.Exit(1)
}
