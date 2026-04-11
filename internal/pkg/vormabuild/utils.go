package vormabuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/procutil"
)

/////////////////////////////////////////////////////////////////////
/////// UTILS -- NEW COMMAND
/////////////////////////////////////////////////////////////////////

func new_cmd(ctx context.Context, command string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = procutil.SysProcAttr()
	return cmd
}

/////////////////////////////////////////////////////////////////////
/////// UTILS -- FILE WRITERS
/////////////////////////////////////////////////////////////////////

func write_str_to_file(s string, out_path string) error {
	if err := os.WriteFile(out_path, []byte(s), 0644); err != nil {
		return fmt.Errorf("error writing string to file: %w", err)
	}
	return nil
}

func write_json_to_file[T any](data T, out_path string) error {
	serialized, err := jsonutil.SerializePretty(data)
	if err != nil {
		return fmt.Errorf("error serializing data: %w", err)
	}
	serialized = append(serialized, '\n')
	if err := os.WriteFile(out_path, serialized, 0644); err != nil {
		return fmt.Errorf("error writing data to file: %w", err)
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/////// UTILS -- ENV
/////////////////////////////////////////////////////////////////////

func env_item_str(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}
func env_item_int(key string, value int) string {
	return fmt.Sprintf("%s=%d", key, value)
}
