package wavebuild

type Checkpoint string

func (c Checkpoint) Str() string { return string(c) }

const (
	Checkpoint_1_CycleStart         Checkpoint = "cycle_start"
	Checkpoint_2_PostAssetPipeline  Checkpoint = "post_asset_pipeline"
	Checkpoint_3_GoCompile          Checkpoint = "go_compile"
	Checkpoint_4_PostGoCompile      Checkpoint = "post_go_compile"
	Checkpoint_5_PostServiceRestart Checkpoint = "post_service_restart"
	Checkpoint_6_Cleanup            Checkpoint = "cleanup"
)

// CheckpointOrd is the numeric ordering of a checkpoint (1–6).
// Using a named type prevents accidental misuse of unrelated ints.
type CheckpointOrd int

func CheckpointOrder(c Checkpoint) CheckpointOrd {
	switch c {
	case Checkpoint_1_CycleStart:
		return 1
	case Checkpoint_2_PostAssetPipeline:
		return 2
	case Checkpoint_3_GoCompile:
		return 3
	case Checkpoint_4_PostGoCompile:
		return 4
	case Checkpoint_5_PostServiceRestart:
		return 5
	case Checkpoint_6_Cleanup:
		return 6
	default:
		return -1
	}
}

func is_valid_checkpoint_ord(o CheckpointOrd) bool {
	return o >= 1 && o <= 6
}
