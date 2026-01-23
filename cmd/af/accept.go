// Package main contains the af accept command implementation.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobias/vibefeld/internal/cli"
	"github.com/tobias/vibefeld/internal/render"
	"github.com/tobias/vibefeld/internal/service"
)

// writeJSONOutput marshals data as indented JSON and writes to the command's output.
func writeJSONOutput(cmd *cobra.Command, data interface{}) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func newAcceptCmd() *cobra.Command {
	var acceptAll bool
	var withNote string
	var confirm bool
	var agent string

	cmd := &cobra.Command{
		Use:     "accept [node-id]...",
		GroupID: GroupVerifier,
		Short:   "Accept proof nodes (verifier action)",
		Long: `Accept validates proof nodes, marking them as verified correct.

This is a verifier action that confirms the node's correctness.
The node's epistemic state changes from pending to validated.

You can accept multiple nodes at once:
  af accept 1.1 1.2 1.3    Accept nodes 1.1, 1.2, and 1.3

Use --all to accept all pending nodes:
  af accept --all          Accept all pending nodes

Use --with-note for partial acceptance (accept with a recorded note):
  af accept 1 --with-note "Minor issue but acceptable"

Notes are recorded in the ledger for the audit trail but do not
block acceptance. This allows verifiers to express nuanced feedback.

If you provide --agent, the tool will check if you have raised any
challenges for the node. Accepting without having raised any challenges
requires --confirm to ensure thorough verification.

Examples:
  af accept 1              Accept the root node
  af accept 1.2.3          Accept a specific child node
  af accept 1.1 1.2        Accept multiple nodes at once
  af accept --all          Accept all pending nodes
  af accept -a             Accept all pending nodes (short form)
  af accept 1 --with-note "Consider clarifying step 2"
  af accept 1 -d ./proof   Accept using specific directory
  af accept 1 --agent verifier-1  Accept with agent verification
  af accept 1 --agent v1 --confirm  Accept without having raised challenges

Workflow:
  After accepting, use 'af status' to see the updated proof tree and
  'af progress' to check overall completion. Use 'af jobs' to find the next
  node to verify.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccept(cmd, args, acceptAll, withNote, confirm, agent)
		},
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().StringP("format", "f", "text", "Output format (text/json)")
	cmd.Flags().BoolVarP(&acceptAll, "all", "a", false, "Accept all pending nodes")
	cmd.Flags().StringVar(&withNote, "with-note", "", "Optional acceptance note for partial acceptance")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm acceptance without having raised challenges")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent ID (verifier identity for challenge verification)")

	return cmd
}

// acceptParams holds the parameters for the accept command.
type acceptParams struct {
	dir       string
	format    string
	acceptAll bool
	withNote  string
	confirm   bool
	agent     string
	args      []string
}

// validateAcceptInput validates the accept command input and returns any usage error.
func validateAcceptInput(params acceptParams) error {
	hasNodeIDs := len(params.args) > 0

	if params.acceptAll && hasNodeIDs {
		return render.NewUsageError("af accept",
			"--all and node IDs are mutually exclusive; use one or the other",
			[]string{"af accept --all", "af accept 1.1 1.2 1.3"})
	}

	if !params.acceptAll && !hasNodeIDs {
		return render.NewUsageError("af accept",
			"either specify node IDs or use --all to accept all pending nodes",
			[]string{"af accept 1.1", "af accept 1.1 1.2 1.3", "af accept --all"})
	}

	if params.withNote != "" && (params.acceptAll || len(params.args) > 1) {
		return render.NewUsageError("af accept",
			"--with-note can only be used when accepting a single node",
			[]string{"af accept 1 --with-note \"Minor issue but acceptable\""})
	}

	return nil
}

// getNodeIDsToAccept collects the node IDs to accept, either from args or pending nodes.
// Returns nil if --all was used but there are no pending nodes (after outputting appropriate message).
func getNodeIDsToAccept(cmd *cobra.Command, svc *service.ProofService, params acceptParams) ([]service.NodeID, error) {
	examples := render.GetExamples("af accept")

	if params.acceptAll {
		pendingNodes, err := svc.LoadPendingNodes()
		if err != nil {
			return nil, fmt.Errorf("error loading pending nodes: %w", err)
		}

		if len(pendingNodes) == 0 {
			outputNoPendingNodes(cmd, params.format)
			return nil, nil
		}

		nodeIDs := make([]service.NodeID, len(pendingNodes))
		for i, n := range pendingNodes {
			nodeIDs[i] = n.ID
		}
		return nodeIDs, nil
	}

	nodeIDs := make([]service.NodeID, 0, len(params.args))
	for _, nodeIDStr := range params.args {
		nodeID, err := service.ParseNodeID(nodeIDStr)
		if err != nil {
			return nil, render.InvalidNodeIDError("af accept", nodeIDStr, examples)
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs, nil
}

// outputNoPendingNodes outputs the "no pending nodes" message in the appropriate format.
func outputNoPendingNodes(cmd *cobra.Command, format string) {
	switch strings.ToLower(format) {
	case "json":
		_ = writeJSONOutput(cmd, map[string]interface{}{
			"accepted": []string{},
			"message":  "no pending nodes to accept",
		})
	default:
		fmt.Fprintln(cmd.OutOrStdout(), "No pending nodes to accept.")
	}
}

// verifyAgentChallenges checks that the agent has raised challenges for all nodes.
func verifyAgentChallenges(svc *service.ProofService, nodeIDs []service.NodeID, agent string, confirm bool) error {
	if agent == "" || confirm {
		return nil
	}

	st, err := svc.LoadState()
	if err != nil {
		return fmt.Errorf("error loading state: %w", err)
	}

	for _, nodeID := range nodeIDs {
		if !st.VerifierRaisedChallengeForNode(nodeID, agent) {
			return fmt.Errorf("you haven't raised any challenges for node %s; use --confirm to accept anyway", nodeID.String())
		}
	}
	return nil
}

// performSingleAcceptance handles acceptance of a single node.
func performSingleAcceptance(cmd *cobra.Command, svc *service.ProofService, nodeID service.NodeID, withNote, format string) error {
	var acceptErr error
	if withNote != "" {
		acceptErr = svc.AcceptNodeWithNote(nodeID, withNote)
	} else {
		acceptErr = svc.AcceptNode(nodeID)
	}
	if acceptErr != nil {
		if errors.Is(acceptErr, service.ErrBlockingChallenges) {
			return handleBlockingChallengesError(cmd, svc, nodeID, format, acceptErr)
		}
		return fmt.Errorf("error accepting node: %w", acceptErr)
	}

	st, stateErr := svc.LoadState()
	var summary verificationSummary
	if stateErr == nil {
		summary = getVerificationSummary(st, nodeID, withNote)
	}

	return outputSingleAcceptance(cmd, nodeID, withNote, format, summary, stateErr == nil)
}

// outputSingleAcceptance outputs the result of a single node acceptance.
func outputSingleAcceptance(cmd *cobra.Command, nodeID service.NodeID, withNote, format string, summary verificationSummary, hasSummary bool) error {
	switch strings.ToLower(format) {
	case "json":
		result := map[string]interface{}{
			"node_id":  nodeID.String(),
			"status":   "validated",
			"accepted": true,
		}
		if withNote != "" {
			result["note"] = withNote
		}

		if hasSummary {
			verificationSummaryJSON := map[string]interface{}{
				"challenges_raised":   summary.ChallengesRaised,
				"challenges_resolved": summary.ChallengesResolved,
			}

			if len(summary.Dependencies) > 0 {
				deps := make([]map[string]string, len(summary.Dependencies))
				for i, dep := range summary.Dependencies {
					deps[i] = map[string]string{
						"id":     dep.ID,
						"status": dep.Status,
					}
				}
				verificationSummaryJSON["dependencies"] = deps
			}

			result["verification_summary"] = verificationSummaryJSON
		}

		return writeJSONOutput(cmd, result)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Node %s accepted and validated.\n", nodeID.String())
		if hasSummary {
			outputVerificationSummaryText(cmd, nodeID, summary)
		}
	}
	return nil
}

// performBulkAcceptance handles acceptance of multiple nodes.
func performBulkAcceptance(cmd *cobra.Command, svc *service.ProofService, nodeIDs []service.NodeID, format string) error {
	if err := svc.AcceptNodeBulk(nodeIDs); err != nil {
		if errors.Is(err, service.ErrBlockingChallenges) {
			nodeID := extractNodeIDFromBlockingError(err)
			if nodeID != nil {
				return handleBlockingChallengesError(cmd, svc, *nodeID, format, err)
			}
		}
		return fmt.Errorf("error accepting nodes: %w", err)
	}

	return outputBulkAcceptance(cmd, service.ToStringSlice(nodeIDs), format)
}

// outputBulkAcceptance outputs the result of a bulk node acceptance.
func outputBulkAcceptance(cmd *cobra.Command, acceptedStrs []string, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return writeJSONOutput(cmd, map[string]interface{}{
			"accepted": acceptedStrs,
			"count":    len(acceptedStrs),
			"status":   "validated",
		})
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Accepted %d nodes:\n", len(acceptedStrs))
		for _, idStr := range acceptedStrs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s - validated\n", idStr)
		}
	}
	return nil
}

func runAccept(cmd *cobra.Command, args []string, acceptAll bool, withNote string, confirm bool, agent string) error {
	dir := cli.MustString(cmd, "dir")
	format := cli.MustString(cmd, "format")

	params := acceptParams{
		dir:       dir,
		format:    format,
		acceptAll: acceptAll,
		withNote:  withNote,
		confirm:   confirm,
		agent:     agent,
		args:      args,
	}

	if err := validateAcceptInput(params); err != nil {
		return err
	}

	svc, err := service.NewProofService(dir)
	if err != nil {
		return fmt.Errorf("error accessing proof directory: %w", err)
	}

	nodeIDs, err := getNodeIDsToAccept(cmd, svc, params)
	if err != nil {
		return err
	}
	if nodeIDs == nil {
		return nil // No pending nodes, already output message
	}

	if err := verifyAgentChallenges(svc, nodeIDs, agent, confirm); err != nil {
		return err
	}

	if len(nodeIDs) == 1 {
		return performSingleAcceptance(cmd, svc, nodeIDs[0], withNote, format)
	}
	return performBulkAcceptance(cmd, svc, nodeIDs, format)
}

// handleBlockingChallengesError displays blocking challenges that prevent acceptance.
// It formats the error output based on the requested format (text or json).
func handleBlockingChallengesError(cmd *cobra.Command, svc *service.ProofService, nodeID service.NodeID, format string, origErr error) error {
	// Load state to get the blocking challenges
	st, err := svc.LoadState()
	if err != nil {
		// If we can't load state, just return the original error
		return fmt.Errorf("error accepting node: %w", origErr)
	}

	blockingChallenges := st.GetBlockingChallengesForNode(nodeID)

	switch strings.ToLower(format) {
	case "json":
		return outputBlockingChallengesJSON(cmd, nodeID, blockingChallenges, origErr)
	default:
		return outputBlockingChallengesText(cmd, nodeID, blockingChallenges, origErr)
	}
}

// outputBlockingChallengesText displays blocking challenges in text format.
func outputBlockingChallengesText(cmd *cobra.Command, nodeID service.NodeID, challenges []*service.Challenge, origErr error) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Cannot accept node %s: blocking challenges must be resolved first.\n\n", nodeID.String()))

	if len(challenges) > 0 {
		sb.WriteString("Blocking Challenges:\n")
		for i, c := range challenges {
			sb.WriteString(fmt.Sprintf("  %d. [%s] %s (severity: %s)\n", i+1, c.ID, c.Target, c.Severity))
			sb.WriteString(fmt.Sprintf("     Reason: %s\n", c.Reason))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("To investigate:\n"))
	sb.WriteString(fmt.Sprintf("  af show %s              View node details and context\n", nodeID.String()))
	sb.WriteString(fmt.Sprintf("  af challenges --node %s  List all challenges on this node\n", nodeID.String()))
	sb.WriteString("\n")
	sb.WriteString("How to resolve:\n")
	sb.WriteString("  - Use 'af refine' to address the challenges by improving the proof\n")
	sb.WriteString("  - Use 'af resolve-challenge <challenge-id>' to mark a challenge resolved\n")
	sb.WriteString("  - Use 'af withdraw-challenge <challenge-id>' to withdraw a challenge raised in error\n")

	// Print the output
	fmt.Fprint(cmd.OutOrStdout(), sb.String())

	// Return error with just the summary (without the details we already printed)
	return fmt.Errorf("node %s has %d blocking challenge(s)", nodeID.String(), len(challenges))
}

// outputBlockingChallengesJSON displays blocking challenges in JSON format.
func outputBlockingChallengesJSON(cmd *cobra.Command, nodeID service.NodeID, challenges []*service.Challenge, origErr error) error {
	type challengeInfo struct {
		ID       string `json:"id"`
		Target   string `json:"target"`
		Severity string `json:"severity"`
		Reason   string `json:"reason"`
	}

	type blockingResponse struct {
		Error              string          `json:"error"`
		NodeID             string          `json:"node_id"`
		BlockingChallenges []challengeInfo `json:"blocking_challenges"`
		HowToResolve       []string        `json:"how_to_resolve"`
	}

	challengeList := make([]challengeInfo, len(challenges))
	for i, c := range challenges {
		challengeList[i] = challengeInfo{
			ID:       c.ID,
			Target:   c.Target,
			Severity: c.Severity,
			Reason:   c.Reason,
		}
	}

	response := blockingResponse{
		Error:              "blocking_challenges",
		NodeID:             nodeID.String(),
		BlockingChallenges: challengeList,
		HowToResolve: []string{
			"Use 'af refine' to address the challenges by improving the proof",
			"Use 'af resolve <challenge-id>' to resolve a challenge with an explanation",
			"Use 'af withdraw <challenge-id>' to withdraw a challenge if it was raised in error",
		},
	}

	if err := writeJSONOutput(cmd, response); err != nil {
		return err
	}

	// Return error with just the summary
	return fmt.Errorf("node %s has %d blocking challenge(s)", nodeID.String(), len(challenges))
}

// verificationSummary contains information about challenges and dependencies
// for a node that was just accepted.
type verificationSummary struct {
	ChallengesRaised   int
	ChallengesResolved int
	Dependencies       []dependencyInfo
	Note               string
}

// dependencyInfo contains the ID and status of a dependency.
type dependencyInfo struct {
	ID     string
	Status string
}

// lookupContextStatus determines the status of a context item by checking
// if it's a definition, assumption, external, or lemma.
func lookupContextStatus(st *service.State, ctx string) string {
	if st.GetDefinition(ctx) != nil || st.GetDefinitionByName(ctx) != nil {
		return "definition"
	}
	if st.GetAssumption(ctx) != nil {
		return "assumed"
	}
	if st.GetExternal(ctx) != nil || st.GetExternalByName(ctx) != nil {
		return "external"
	}
	if st.GetLemma(ctx) != nil {
		return "lemma"
	}
	return "unknown"
}

// getVerificationSummary retrieves challenge and dependency information for a node.
func getVerificationSummary(st *service.State, nodeID service.NodeID, note string) verificationSummary {
	summary := verificationSummary{
		Note: note,
	}

	// Count challenges raised and resolved for this node
	nodeIDStr := nodeID.String()
	for _, c := range st.AllChallenges() {
		if c.NodeID.String() == nodeIDStr {
			summary.ChallengesRaised++
			if c.Status == service.ChallengeStatusResolved {
				summary.ChallengesResolved++
			}
		}
	}

	// Get dependency information
	n := st.GetNode(nodeID)
	if n != nil {
		// Add regular dependencies
		for _, depID := range n.Dependencies {
			depNode := st.GetNode(depID)
			status := "unknown"
			if depNode != nil {
				status = string(depNode.EpistemicState)
			}
			summary.Dependencies = append(summary.Dependencies, dependencyInfo{
				ID:     depID.String(),
				Status: status,
			})
		}

		// Add context items (definitions, assumptions, externals)
		for _, ctx := range n.Context {
			summary.Dependencies = append(summary.Dependencies, dependencyInfo{
				ID:     ctx,
				Status: lookupContextStatus(st, ctx),
			})
		}
	}

	return summary
}

// outputVerificationSummaryText outputs the verification summary in text format.
func outputVerificationSummaryText(cmd *cobra.Command, nodeID service.NodeID, summary verificationSummary) {
	fmt.Fprintf(cmd.OutOrStdout(), "\nVerification summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Challenges: %d raised, %d resolved\n",
		summary.ChallengesRaised, summary.ChallengesResolved)

	if len(summary.Dependencies) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Dependencies:\n")
		for _, dep := range summary.Dependencies {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", dep.ID, dep.Status)
		}
	}

	if summary.Note != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Note: %s\n", summary.Note)
	}
}

// extractNodeIDFromBlockingError attempts to extract the node ID from a blocking challenges error message.
// Returns nil if the node ID cannot be extracted.
func extractNodeIDFromBlockingError(err error) *service.NodeID {
	if err == nil {
		return nil
	}

	// The error format from service is: "node has unresolved blocking challenges: node X has N blocking challenge(s): ..."
	errStr := err.Error()

	// Look for "node " followed by the ID
	const nodePrefix = "node "
	idx := strings.Index(errStr, nodePrefix)
	if idx == -1 {
		return nil
	}

	// Skip past "node "
	remaining := errStr[idx+len(nodePrefix):]

	// Find the end of the node ID (space or " has")
	endIdx := strings.Index(remaining, " has")
	if endIdx == -1 {
		endIdx = strings.Index(remaining, " ")
	}
	if endIdx == -1 {
		return nil
	}

	nodeIDStr := remaining[:endIdx]
	nodeID, err := service.ParseNodeID(nodeIDStr)
	if err != nil {
		return nil
	}

	return &nodeID
}

func init() {
	rootCmd.AddCommand(newAcceptCmd())
}
