package vormabuild

import (
	"context"
	"fmt"
	"os"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/searchparams"
)

type live_state struct {
	Error         string
	VormaConfig   *vorma.Config
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

func get_live_state(v *vorma.Router) (*live_state, error) {
	cfg, err := to_cfg(v.Instance().Config())
	if err != nil {
		return nil, fmt.Errorf("error converting config: %w", err)
	}

	ts_result, err := cfg.to_live_ts_result(v)
	if err != nil {
		return nil, fmt.Errorf("error generating TS types: %w", err)
	}

	ts_modules, err := cfg.get_dev_ts_modules(v)
	if err != nil {
		return nil, fmt.Errorf("error getting TS modules: %w", err)
	}

	views := v.Views()

	search_schemas := make(map[string]searchparams.Schema, len(views))
	for _, view := range views {
		pattern := view.GetPattern()
		if pattern == "" {
			continue
		}
		schema, err := searchparams.SchemaFromValue(view.IType().Instance)
		if err != nil {
			return nil, fmt.Errorf(
				"error generating view search schema for %s: %w",
				pattern,
				err,
			)
		}
		search_schemas[pattern] = schema
	}

	ls := &live_state{
		VormaConfig:   cfg.C,
		TSResult:      ts_result,
		TSModules:     ts_modules,
		SearchSchemas: search_schemas,
	}

	return ls, nil
}

func print_live_state_and_exit(v *vorma.Router) {
	ls, err := get_live_state(v)
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
