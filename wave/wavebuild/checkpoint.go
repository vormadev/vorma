package wavebuild

type Checkpoint string

func (c Checkpoint) Str() string { return string(c) }

const (
	Checkpoint_1_CycleStart                 Checkpoint = "cycle_start"
	Checkpoint_2_UserlandPublicFilemapReady Checkpoint = "userland_public_filemap_ready"
	Checkpoint_3_FullPublicFilemapFinalized Checkpoint = "full_public_filemap_finalized"
	Checkpoint_4_GoCompile                  Checkpoint = "go_compile"
	Checkpoint_5_GoCompileComplete          Checkpoint = "go_compile_complete"
	Checkpoint_6_ServiceRestarted           Checkpoint = "service_restarted"
	Checkpoint_7_CycleEnd                   Checkpoint = "cycle_end"
)

// CheckpointOrd is the numeric ordering of a checkpoint (1–7).
// Using a named type prevents accidental misuse of unrelated ints.
type CheckpointOrd int

func CheckpointOrder(c Checkpoint) CheckpointOrd {
	switch c {
	case Checkpoint_1_CycleStart:
		return 1
	case Checkpoint_2_UserlandPublicFilemapReady:
		return 2
	case Checkpoint_3_FullPublicFilemapFinalized:
		return 3
	case Checkpoint_4_GoCompile:
		return 4
	case Checkpoint_5_GoCompileComplete:
		return 5
	case Checkpoint_6_ServiceRestarted:
		return 6
	case Checkpoint_7_CycleEnd:
		return 7
	default:
		return -1
	}
}

func is_valid_checkpoint_ord(o CheckpointOrd) bool {
	return o >= 1 && o <= 7
}
