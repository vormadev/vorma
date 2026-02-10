package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vormadev/vorma/spec/tools/internal/specutil"
)

type reviewPass struct {
	ReviewerOwner     string           `json:"reviewer_owner"`
	ReviewerClaimSlot string           `json:"reviewer_claim_slot"`
	Result            string           `json:"result"`
	ArtifactsHash     string           `json:"artifacts_hash"`
	CompletedUTC      string           `json:"completed_utc"`
	NotesRef          string           `json:"notes_ref"`
	NotesLog          []map[string]any `json:"notes_log"`
}

type reviewGate struct {
	MinerOwner          string     `json:"miner_owner"`
	TargetArtifactsHash string     `json:"target_artifacts_hash"`
	Pass1               reviewPass `json:"pass1"`
	Pass2               reviewPass `json:"pass2"`
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./spec/tools/cmd/record_review_pass SLOT-XXX <reviewer_owner> <reviewer_claim_slot> <pass(1|2)> <PASS_NO_NOTES|FAIL_NOTES> <notes_ref>")
	os.Exit(1)
}

func passPending() reviewPass {
	return reviewPass{
		ReviewerOwner:     "-",
		ReviewerClaimSlot: "-",
		Result:            "PENDING",
		ArtifactsHash:     "-",
		CompletedUTC:      "-",
		NotesRef:          "-",
		NotesLog:          []map[string]any{},
	}
}

func normalizePass(p reviewPass) reviewPass {
	if p.ReviewerOwner == "" {
		p.ReviewerOwner = "-"
	}
	if p.ReviewerClaimSlot == "" {
		p.ReviewerClaimSlot = "-"
	}
	if p.Result == "" {
		p.Result = "PENDING"
	}
	if p.ArtifactsHash == "" {
		p.ArtifactsHash = "-"
	}
	if p.CompletedUTC == "" {
		p.CompletedUTC = "-"
	}
	if p.NotesRef == "" {
		p.NotesRef = "-"
	}
	if p.NotesLog == nil {
		p.NotesLog = []map[string]any{}
	}
	return p
}

func main() {
	args := os.Args[1:]
	if len(args) != 6 {
		usage()
	}

	slotID := args[0]
	reviewerOwner := args[1]
	reviewerClaimSlot := args[2]
	passIndex := args[3]
	result := args[4]
	notesRef := args[5]

	if reviewerOwner == "-" {
		fmt.Fprintln(os.Stderr, "reviewer_owner '-' is reserved")
		os.Exit(1)
	}
	if reviewerClaimSlot == "-" {
		fmt.Fprintln(os.Stderr, "reviewer_claim_slot '-' is reserved")
		os.Exit(1)
	}
	if passIndex != "1" && passIndex != "2" {
		fmt.Fprintln(os.Stderr, "pass index must be 1 or 2")
		os.Exit(1)
	}
	if result != "PASS_NO_NOTES" && result != "FAIL_NOTES" {
		fmt.Fprintln(os.Stderr, "result must be PASS_NO_NOTES or FAIL_NOTES")
		os.Exit(1)
	}
	if result == "PASS_NO_NOTES" && notesRef != "-" {
		fmt.Fprintln(os.Stderr, "notes_ref must be '-' when result is PASS_NO_NOTES")
		os.Exit(1)
	}
	if result == "FAIL_NOTES" && notesRef == "-" {
		fmt.Fprintln(os.Stderr, "notes_ref is required when result is FAIL_NOTES")
		os.Exit(1)
	}

	currentBranch, err := specutil.CurrentBranch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "detect current branch: %v\n", err)
		os.Exit(1)
	}
	currentClaimContext, err := specutil.CurrentClaimContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "detect current claim context: %v\n", err)
		os.Exit(1)
	}

	lockPath, err := specutil.DispatchLockPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	err = specutil.WithLock(lockPath, func() error {
		doc, err := specutil.ParseDispatchFile("spec/MINING_DISPATCH.json")
		if err != nil {
			return err
		}

		_, slotRow := specutil.FindRowBySlotID(doc.Rows, slotID)
		if slotRow == nil {
			return fmt.Errorf("slot not found: %s", slotID)
		}
		if slotRow.Status != "CLAIMED" && slotRow.Status != "DONE" {
			return fmt.Errorf("slot must be CLAIMED or DONE to record review pass: %s (%s)", slotID, slotRow.Status)
		}
		if slotRow.Owner == "-" {
			return fmt.Errorf("slot owner is invalid for %s", slotID)
		}
		if slotRow.ClaimBranch == "-" || slotRow.ClaimContext == "-" {
			return fmt.Errorf("mined slot is missing claim branch/context metadata: %s", slotID)
		}

		_, reviewerRow := specutil.FindRowBySlotID(doc.Rows, reviewerClaimSlot)
		if reviewerRow == nil {
			return fmt.Errorf("reviewer claim slot not found: %s", reviewerClaimSlot)
		}
		if reviewerRow.Status != "CLAIMED" && reviewerRow.Status != "DONE" {
			return fmt.Errorf("reviewer claim slot must be CLAIMED or DONE: %s (%s)", reviewerClaimSlot, reviewerRow.Status)
		}
		if reviewerRow.Owner != reviewerOwner {
			return fmt.Errorf("reviewer owner mismatch for claim slot: expected %s, got %s", reviewerRow.Owner, reviewerOwner)
		}
		if reviewerClaimSlot == slotID {
			return fmt.Errorf("reviewer claim slot must differ from mined slot: %s", slotID)
		}
		if reviewerOwner == slotRow.Owner {
			return fmt.Errorf("reviewer owner must be independent from miner owner: %s", reviewerOwner)
		}
		if reviewerRow.ClaimBranch == "-" || reviewerRow.ClaimContext == "-" {
			return fmt.Errorf("reviewer claim slot is missing claim branch/context metadata: %s", reviewerClaimSlot)
		}
		if reviewerRow.ClaimContext == slotRow.ClaimContext {
			return fmt.Errorf("reviewer claim context must differ from mined slot claim context")
		}
		if reviewerRow.ClaimBranch != currentBranch {
			return fmt.Errorf("record_review_pass must run from reviewer claim branch %q (current: %q)", reviewerRow.ClaimBranch, currentBranch)
		}
		if reviewerRow.ClaimContext != currentClaimContext {
			return fmt.Errorf("record_review_pass must run from the reviewer claim worktree context for slot %s", reviewerClaimSlot)
		}

		specPath := slotRow.SpecPath
		specFile := specPath + "/spec.json"
		content, err := os.ReadFile(specFile)
		if err != nil {
			return fmt.Errorf("missing spec file: %s", specFile)
		}

		hash, err := specutil.ComputeSpecHash(specPath)
		if err != nil {
			return err
		}

		var spec map[string]any
		if err := json.Unmarshal(content, &spec); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", specFile, err)
		}

		rawGate, ok := spec["review_gate"].(map[string]any)
		if !ok {
			rawGate = map[string]any{}
		}

		gateJSON, err := json.Marshal(rawGate)
		if err != nil {
			return err
		}
		gate := reviewGate{}
		if err := json.Unmarshal(gateJSON, &gate); err != nil {
			return err
		}
		gate.Pass1 = normalizePass(gate.Pass1)
		gate.Pass2 = normalizePass(gate.Pass2)

		if gate.MinerOwner != "" && gate.MinerOwner != "-" && gate.MinerOwner != slotRow.Owner {
			return fmt.Errorf("review_gate.miner_owner mismatch: expected %s, found %s", slotRow.Owner, gate.MinerOwner)
		}
		gate.MinerOwner = slotRow.Owner
		gate.TargetArtifactsHash = hash

		now := specutil.NowUTC()
		apply := func(p *reviewPass) {
			p.ReviewerOwner = reviewerOwner
			p.ReviewerClaimSlot = reviewerClaimSlot
			p.Result = result
			p.ArtifactsHash = hash
			p.CompletedUTC = now
			p.NotesRef = notesRef
			if result == "FAIL_NOTES" {
				p.NotesLog = []map[string]any{{
					"at_utc":    now,
					"notes_ref": notesRef,
					"comment":   "Review notes recorded.",
				}}
			} else {
				p.NotesLog = []map[string]any{}
			}
		}

		if passIndex == "1" {
			apply(&gate.Pass1)
			if result == "FAIL_NOTES" {
				gate.Pass2 = passPending()
			}
		} else {
			if gate.Pass1.Result != "PASS_NO_NOTES" {
				return fmt.Errorf("cannot record pass 2 before pass 1 is PASS_NO_NOTES")
			}
			if gate.Pass1.ArtifactsHash != hash {
				return fmt.Errorf("cannot record pass 2: pass 1 hash (%s) does not match current artifacts (%s)", gate.Pass1.ArtifactsHash, hash)
			}
			if gate.Pass1.ReviewerOwner == reviewerOwner {
				return fmt.Errorf("pass 2 reviewer_owner must differ from pass 1 reviewer_owner")
			}
			if gate.Pass1.ReviewerClaimSlot == reviewerClaimSlot {
				return fmt.Errorf("pass 2 reviewer_claim_slot must differ from pass 1 reviewer_claim_slot")
			}
			_, pass1Row := specutil.FindRowBySlotID(doc.Rows, gate.Pass1.ReviewerClaimSlot)
			if pass1Row == nil {
				return fmt.Errorf("pass 1 reviewer_claim_slot not found in dispatch: %s", gate.Pass1.ReviewerClaimSlot)
			}
			if pass1Row.ClaimContext == "-" {
				return fmt.Errorf("pass 1 reviewer_claim_slot missing claim context metadata: %s", gate.Pass1.ReviewerClaimSlot)
			}
			if pass1Row.ClaimContext == reviewerRow.ClaimContext {
				return fmt.Errorf("pass 2 reviewer claim context must differ from pass 1 reviewer claim context")
			}
			apply(&gate.Pass2)
		}

		encodedGate, err := json.Marshal(gate)
		if err != nil {
			return err
		}
		var gateObj map[string]any
		if err := json.Unmarshal(encodedGate, &gateObj); err != nil {
			return err
		}
		spec["review_gate"] = gateObj

		updated, err := json.MarshalIndent(spec, "", "\t")
		if err != nil {
			return err
		}
		if err := os.WriteFile(specFile, append(updated, '\n'), 0o644); err != nil {
			return err
		}

		fmt.Printf("recorded review pass %s for %s (%s): %s\n", passIndex, slotID, specPath, result)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
