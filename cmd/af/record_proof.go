package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// newRecordProofCmd creates the `af record-proof` command: the atomic
// prover-write kernel op an external driver (rk) uses to record a proof step on
// a CHALLENGED node. Unlike claim-then-refine, it does the whole hand-off in
// one transition — verify prover-job classification + expected hash, create the
// children, dispose the open challenge(s), and release the claim — so the node
// cycles back into verifier territory instead of staying prover-classified and
// claimed forever (rk B1 + FU3).
func newRecordProofCmd() *cobra.Command {
	var owner string
	var dir string
	var format string
	var childrenJSON string
	var expectHash string

	cmd := &cobra.Command{
		Use:     "record-proof <parent-id>",
		GroupID: GroupProver,
		Short:   "Atomically record a proof step: refine + dispose challenge + release",
		Long: `Record a prover's decomposition of a CHALLENGED node in one atomic step.

record-proof is the driver-facing prover write. The target node must currently
be a PROVER JOB (it has an open blocking challenge, or is draft/needs_refinement)
— a fresh conjecture with no challenge is a verifier job and is refused. In a
single CAS-protected transition it:

  1. verifies the node is a prover job (matching 'af export --graph json's
     prover_ready flag);
  2. if --expect-hash is given, verifies it still matches the node's content
     hash (refusing a proof generated for bytes that changed mid-turn);
  3. refuses a node claimed by a different owner;
  4. creates the children from --children (each may carry per-child "depends");
  5. resolves every open challenge on the node;
  6. releases the node if --owner holds its claim.

The children become the new verifier frontier; once they clear bottom-up, the
node itself becomes verifier-ready again.

Examples:
  af record-proof 1 -o prover-1 --children '[{"statement":"Step A"},{"statement":"Uses A","depends":["#0"]}]'
  af record-proof 1.2 -o prover-1 --expect-hash <hash> --children '[{"statement":"..."}]' -f json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordProof(cmd, args[0], owner, dir, format, childrenJSON, expectHash)
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Prover identity (required; recorded as each child's author)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Proof directory")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format (text/json)")
	cmd.Flags().StringVar(&childrenJSON, "children", "", "JSON array of child specs (required): [{\"statement\":...,\"type\"?:...,\"justification\"?:...,\"depends\"?:[...]}]")
	cmd.Flags().StringVar(&expectHash, "expect-hash", "", "Content hash the proof was authored against; refused if the node changed since")

	return cmd
}

func runRecordProof(cmd *cobra.Command, nodeIDStr, owner, dir, format, childrenJSON, expectHash string) error {
	examples := render.GetExamples("af record-proof")

	if strings.TrimSpace(owner) == "" {
		return render.MissingFlagError("af record-proof", "owner", examples)
	}
	if strings.TrimSpace(childrenJSON) == "" {
		return render.NewUsageError("af record-proof", "--children is required (JSON array of child specs)", examples)
	}

	parentID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return render.InvalidNodeIDError("af record-proof", nodeIDStr, examples)
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("failed to open proof: %w", err)
	}
	st, err := svc.LoadState()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("proof not initialized. Run 'af init' first")
		}
		return fmt.Errorf("failed to load state: %w", err)
	}

	var children []childSpec
	if err := json.Unmarshal([]byte(childrenJSON), &children); err != nil {
		return render.NewUsageError("af record-proof", fmt.Sprintf("invalid JSON for --children: %v", err), examples)
	}
	if len(children) == 0 {
		return render.NewUsageError("af record-proof", "--children requires at least one child specification", examples)
	}

	specs := make([]service.ChildSpec, len(children))
	for i, child := range children {
		if strings.TrimSpace(child.Statement) == "" {
			return render.NewUsageError("af record-proof", fmt.Sprintf("child %d: statement is required", i+1), examples)
		}
		childType := child.Type
		if childType == "" {
			childType = "claim"
		}
		childInference := child.Inference
		if childInference == "" {
			childInference = "assumption"
		}
		nodeType, inferenceType, err := validateNodeTypeAndInference("af record-proof", childType, childInference, examples)
		if err != nil {
			return fmt.Errorf("child %d: %w", i+1, err)
		}
		if err := service.ValidateDefCitations(child.Statement, st); err != nil {
			return fmt.Errorf("child %d: invalid definition citation: %v", i+1, err)
		}
		specs[i] = service.ChildSpec{
			NodeType:     nodeType,
			Statement:    child.Statement,
			Inference:    inferenceType,
			Dependencies: child.Depends,
		}
	}

	res, err := svc.RecordProof(service.RecordProofSpec{
		ParentID:   parentID,
		Owner:      owner,
		Children:   specs,
		ExpectHash: expectHash,
	})
	if err != nil {
		return err
	}

	return formatRecordProofOutput(cmd, format, nodeIDStr, res)
}

func formatRecordProofOutput(cmd *cobra.Command, format, parentIDStr string, res *service.RecordProofResult) error {
	childIDs := service.ToStringSlice(res.ChildIDs)
	if format == "json" {
		result := map[string]interface{}{
			"success":             true,
			"parent_id":           parentIDStr,
			"children":            childIDs,
			"resolved_challenges": res.ResolvedChallenges,
			"released":            res.Released,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Recorded proof under %s:\n", parentIDStr)
	fmt.Fprintf(cmd.OutOrStdout(), "  Children: %s\n", strings.Join(childIDs, ", "))
	fmt.Fprintf(cmd.OutOrStdout(), "  Resolved challenges: %s\n", strings.Join(res.ResolvedChallenges, ", "))
	fmt.Fprintf(cmd.OutOrStdout(), "  Released claim: %v\n", res.Released)
	return nil
}

func init() {
	rootCmd.AddCommand(newRecordProofCmd())
}
