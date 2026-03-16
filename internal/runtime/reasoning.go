package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

const (
	reasoningNamespace   = "reasoning"
	reasoningToolkitName = "reasoning_tools"
	reasoningPrefix      = reasoningNamespace + "."

	reasoningThinkName   = "think"
	reasoningAnalyzeName = "analyze"

	reasoningKindArtifact   = "reasoning_artifact"
	reasoningKindDiagnostic = "reasoning_diagnostic"

	internalReasoningPromptPrefix = "Runtime guidance:"
)

// Option mutates internal runtime configuration.
type Option func(*engineOptions)

// ReasoningMode controls which internal reasoning tools are available.
type ReasoningMode string

const (
	// ReasoningModeDisabled disables the internal reasoning toolkit.
	ReasoningModeDisabled ReasoningMode = "disabled"
	// ReasoningModeThinkOnly registers only `reasoning.think`.
	ReasoningModeThinkOnly ReasoningMode = "think_only"
	// ReasoningModeThinkAndAnalyze registers both `reasoning.think` and `reasoning.analyze`.
	ReasoningModeThinkAndAnalyze ReasoningMode = "think_and_analyze"
)

// ReasoningFailurePolicy controls how runtime reasoning failures degrade.
type ReasoningFailurePolicy string

const (
	// ReasoningFailurePolicyFailOpen disables reasoning for the run and returns to the base loop.
	ReasoningFailurePolicyFailOpen ReasoningFailurePolicy = "fail_open"
	// ReasoningFailurePolicyFailClosed aborts the run on reasoning failures.
	ReasoningFailurePolicyFailClosed ReasoningFailurePolicy = "fail_closed"
)

// ReasoningConfig configures the internal reasoning toolkit for the engine.
//
// Callers should normally start from DefaultReasoningConfig and override only
// the fields they need.
type ReasoningConfig struct {
	Enabled                       bool
	Mode                          ReasoningMode
	MaxReasoningSteps             int
	MaxAnalyzePasses              int
	RequireAnalyzeBeforeResponse  bool
	AnalyzeAfterToolResult        bool
	StoreArtifactsInWorkingMemory bool
	EmitInternalDiagnostics       bool
	FailurePolicy                 ReasoningFailurePolicy
}

// DefaultReasoningConfig returns the spec-aligned internal defaults.
func DefaultReasoningConfig() ReasoningConfig {
	return ReasoningConfig{
		Mode:                          ReasoningModeDisabled,
		MaxReasoningSteps:             2,
		MaxAnalyzePasses:              1,
		StoreArtifactsInWorkingMemory: true,
		FailurePolicy:                 ReasoningFailurePolicyFailOpen,
	}
}

type engineOptions struct {
	reasoning ReasoningConfig
}

func defaultEngineOptions() engineOptions {
	return engineOptions{
		reasoning: DefaultReasoningConfig(),
	}
}

// WithReasoningConfig enables or disables the internal reasoning toolkit.
func WithReasoningConfig(cfg ReasoningConfig) Option {
	return func(opts *engineOptions) {
		opts.reasoning = normalizeReasoningConfig(cfg)
	}
}

type reasoningRuntime struct {
	config   ReasoningConfig
	registry tool.Registry
}

type reasoningRun struct {
	runtime       *reasoningRuntime
	working       memory.WorkingSet
	stepsUsed     int
	analyzePasses int
	disabled      bool
}

type reasoningActionKind string

const (
	reasoningActionContinue reasoningActionKind = "continue"
	reasoningActionCallTool reasoningActionKind = "call_tool"
	reasoningActionRespond  reasoningActionKind = "respond"
)

type reasoningAction struct {
	kind         reasoningActionKind
	toolName     string
	toolInput    map[string]any
	toolResponse string
	note         string
}

type thinkArtifact struct {
	Decision           string
	Plan               []any
	Reason             string
	CandidateToolName  string
	CandidateToolInput map[string]any
	CandidateResponse  string
	NeedsValidation    bool
}

type analyzeArtifact struct {
	Verdict            string
	RecommendedAction  string
	Findings           []any
	Reason             string
	CandidateToolName  string
	CandidateToolInput map[string]any
	CandidateResponse  string
}

func normalizeReasoningConfig(cfg ReasoningConfig) ReasoningConfig {
	out := cfg

	if out.Mode == "" {
		if out.Enabled {
			out.Mode = ReasoningModeThinkAndAnalyze
		} else {
			out.Mode = ReasoningModeDisabled
		}
	}
	if out.Mode != ReasoningModeDisabled {
		out.Enabled = true
	}
	if !out.Enabled {
		out.Mode = ReasoningModeDisabled
	}

	if out.MaxReasoningSteps <= 0 {
		out.MaxReasoningSteps = DefaultReasoningConfig().MaxReasoningSteps
	}
	if out.MaxAnalyzePasses <= 0 {
		out.MaxAnalyzePasses = DefaultReasoningConfig().MaxAnalyzePasses
	}
	if out.MaxAnalyzePasses > out.MaxReasoningSteps {
		out.MaxAnalyzePasses = out.MaxReasoningSteps
	}
	if out.FailurePolicy == "" {
		out.FailurePolicy = ReasoningFailurePolicyFailOpen
	}
	if out.RequireAnalyzeBeforeResponse || out.AnalyzeAfterToolResult {
		out.Enabled = true
		out.Mode = ReasoningModeThinkAndAnalyze
	}
	if out.RequireAnalyzeBeforeResponse {
		out.FailurePolicy = ReasoningFailurePolicyFailClosed
	}

	return out
}

func newReasoningRuntime(cfg ReasoningConfig) (*reasoningRuntime, error) {
	cfg = normalizeReasoningConfig(cfg)
	if !cfg.Enabled || cfg.Mode == ReasoningModeDisabled {
		return nil, nil
	}

	registry := tool.NewRegistry()
	if err := registry.RegisterToolkits(newReasoningToolkit(cfg)); err != nil {
		return nil, err
	}

	return &reasoningRuntime{
		config:   cfg,
		registry: registry,
	}, nil
}

func newReasoningRun(runtime *reasoningRuntime, working memory.WorkingSet) *reasoningRun {
	if runtime == nil {
		return nil
	}
	return &reasoningRun{
		runtime: runtime,
		working: working,
	}
}

func (r *reasoningRun) enabled() bool {
	return r != nil && r.runtime != nil && !r.disabled
}

func (r *reasoningRun) afterToolResult(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	conversation []types.Message,
	effectiveTools map[string]tool.Tool,
	stepIndex int,
	record agent.ToolCallRecord,
) (reasoningAction, error) {
	if !r.enabled() {
		return reasoningAction{kind: reasoningActionContinue}, nil
	}

	artifact, err := r.invokeThink(ctx, definition, req, conversation, effectiveTools, thinkInput{
		StepIndex:      stepIndex,
		LastToolResult: toolCallRecordToMap(record),
	})
	if err != nil {
		return r.onFailure(err, reasoningAction{kind: reasoningActionContinue})
	}

	action, err := r.interpretThink(artifact, record.Output.Content)
	if err != nil {
		return r.onFailure(err, reasoningAction{kind: reasoningActionContinue})
	}
	if action.kind != "" && action.kind != reasoningActionContinue {
		if action.kind == reasoningActionRespond {
			return r.onFailure(fmt.Errorf("reasoning respond is invalid after tool result"), reasoningAction{kind: reasoningActionContinue})
		}
		return action, nil
	}

	if artifact.Decision == "validate" {
		analyze, analyzeErr := r.invokeAnalyze(ctx, definition, req, conversation, analyzeInput{
			StepIndex: stepIndex,
			Candidate: map[string]any{
				"kind":         "tool_result",
				"tool_call_id": record.ID,
				"tool_name":    record.Name,
				"tool_input":   cloneMap(record.Input),
				"tool_output":  toolResultToMap(record.Output),
			},
			Checks: []any{"tool_relevance", "consistency"},
		})
		if analyzeErr != nil {
			return r.onFailure(analyzeErr, reasoningAction{kind: reasoningActionContinue})
		}

		action, err = r.interpretAnalyze(analyze, "")
		if err != nil {
			return r.onFailure(err, reasoningAction{kind: reasoningActionContinue})
		}
		if action.kind == reasoningActionRespond {
			return r.onFailure(fmt.Errorf("reasoning respond is invalid after tool result"), reasoningAction{kind: reasoningActionContinue})
		}
		if analyze.Verdict == "blocked" {
			return reasoningAction{}, fmt.Errorf("reasoning blocked progress after tool result: %s", analyze.Reason)
		}
		return action, nil
	}

	return action, nil
}

func (r *reasoningRun) beforeFinalResponse(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	conversation []types.Message,
	effectiveTools map[string]tool.Tool,
	modelResponse agent.ModelResponse,
	toolCalls []agent.ToolCallRecord,
	message types.Message,
	stepIndex int,
) (reasoningAction, error) {
	fallback := reasoningAction{
		kind:         reasoningActionRespond,
		toolResponse: message.Content,
	}
	if !r.enabled() {
		return fallback, nil
	}

	artifact, err := r.invokeThink(ctx, definition, req, conversation, effectiveTools, thinkInput{
		StepIndex:         stepIndex,
		LastModelOutput:   modelResponseToMap(modelResponse),
		CandidateResponse: message.Content,
		LastToolResult:    lastToolCallToMap(toolCalls),
	})
	if err != nil {
		return r.onFailure(err, fallback)
	}

	action, err := r.interpretThink(artifact, message.Content)
	if err != nil {
		return r.onFailure(err, fallback)
	}

	switch artifact.Decision {
	case "continue":
		return action, nil
	case "call_tool":
		return action, nil
	case "respond":
		return action, nil
	case "fail":
		return r.onFailure(fmt.Errorf("reasoning decided to fail: %s", artifact.Reason), fallback)
	case "validate":
	default:
		return r.onFailure(fmt.Errorf("unsupported reasoning decision %q", artifact.Decision), fallback)
	}

	analyze, analyzeErr := r.invokeAnalyze(ctx, definition, req, conversation, analyzeInput{
		StepIndex: stepIndex,
		Candidate: map[string]any{
			"kind":      "candidate_response",
			"message":   messageToMap(message),
			"response":  message.Content,
			"tool_call": lastToolCallToMap(toolCalls),
		},
		Checks:                []any{"consistency", "completeness", "budget"},
		SupportingToolResults: toolCallsToMaps(toolCalls),
	})
	if analyzeErr != nil {
		return r.onFailure(analyzeErr, fallback)
	}

	action, err = r.interpretAnalyze(analyze, message.Content)
	if err != nil {
		return r.onFailure(err, fallback)
	}
	if analyze.Verdict == "blocked" {
		return reasoningAction{}, fmt.Errorf("reasoning blocked final response: %s", analyze.Reason)
	}
	return action, nil
}

func (r *reasoningRun) onFailure(err error, fallback reasoningAction) (reasoningAction, error) {
	if !r.enabled() {
		return fallback, nil
	}
	if r.runtime.config.FailurePolicy == ReasoningFailurePolicyFailOpen {
		r.disabled = true
		r.recordDiagnostic("reasoning.disabled", map[string]any{"reason": err.Error()})
		return fallback, nil
	}
	return reasoningAction{}, err
}

type thinkInput struct {
	StepIndex         int
	LastModelOutput   map[string]any
	LastToolResult    map[string]any
	CandidateResponse string
}

func (r *reasoningRun) invokeThink(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	conversation []types.Message,
	effectiveTools map[string]tool.Tool,
	input thinkInput,
) (thinkArtifact, error) {
	if err := r.consumeBudget(reasoningThinkName); err != nil {
		return thinkArtifact{}, err
	}

	target, err := r.runtime.registry.Resolve(reasoningPrefix + reasoningThinkName)
	if err != nil {
		return thinkArtifact{}, err
	}

	result, err := tool.Invoke(ctx, target, tool.Call{
		ID:        newToolCallID(req.RunID, input.StepIndex, r.stepsUsed),
		ToolName:  reasoningPrefix + reasoningThinkName,
		RunID:     req.RunID,
		SessionID: req.SessionID,
		AgentID:   definition.Descriptor.ID,
		Input: map[string]any{
			"objective":          deriveObjective(definition, conversation),
			"step_index":         input.StepIndex,
			"max_steps":          req.MaxSteps,
			"messages":           messagesToMaps(conversation),
			"available_tools":    toolSpecsToMaps(effectiveTools),
			"constraints":        constraintsToMap(definition, req),
			"last_model_output":  cloneMap(input.LastModelOutput),
			"last_tool_result":   cloneMap(input.LastToolResult),
			"candidate_response": strings.TrimSpace(input.CandidateResponse),
			"metadata":           metadataToMap(req.Metadata),
		},
		Metadata: types.CloneMetadata(req.Metadata),
	})
	if err != nil {
		return thinkArtifact{}, err
	}

	artifact, err := parseThinkArtifact(result.Value)
	if err != nil {
		return thinkArtifact{}, err
	}
	r.recordArtifact("reasoning.think", thinkArtifactToMap(artifact))
	return artifact, nil
}

type analyzeInput struct {
	StepIndex             int
	Candidate             map[string]any
	Checks                []any
	SupportingToolResults []any
}

func (r *reasoningRun) invokeAnalyze(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	conversation []types.Message,
	input analyzeInput,
) (analyzeArtifact, error) {
	if err := r.consumeBudget(reasoningAnalyzeName); err != nil {
		return analyzeArtifact{}, err
	}
	r.analyzePasses++

	target, err := r.runtime.registry.Resolve(reasoningPrefix + reasoningAnalyzeName)
	if err != nil {
		return analyzeArtifact{}, err
	}

	result, err := tool.Invoke(ctx, target, tool.Call{
		ID:        newToolCallID(req.RunID, input.StepIndex, r.stepsUsed+r.analyzePasses),
		ToolName:  reasoningPrefix + reasoningAnalyzeName,
		RunID:     req.RunID,
		SessionID: req.SessionID,
		AgentID:   definition.Descriptor.ID,
		Input: map[string]any{
			"objective":               deriveObjective(definition, conversation),
			"step_index":              input.StepIndex,
			"max_steps":               req.MaxSteps,
			"candidate":               cloneMap(input.Candidate),
			"checks":                  cloneSlice(input.Checks),
			"messages":                messagesToMaps(conversation),
			"supporting_tool_results": cloneSlice(input.SupportingToolResults),
			"metadata":                metadataToMap(req.Metadata),
		},
		Metadata: types.CloneMetadata(req.Metadata),
	})
	if err != nil {
		return analyzeArtifact{}, err
	}

	artifact, err := parseAnalyzeArtifact(result.Value)
	if err != nil {
		return analyzeArtifact{}, err
	}
	r.recordArtifact("reasoning.analyze", analyzeArtifactToMap(artifact))
	return artifact, nil
}

func (r *reasoningRun) consumeBudget(toolName string) error {
	if !r.enabled() {
		return nil
	}
	if r.stepsUsed >= r.runtime.config.MaxReasoningSteps {
		return fmt.Errorf("reasoning budget exceeded before %s", toolName)
	}
	if toolName == reasoningAnalyzeName && r.analyzePasses >= r.runtime.config.MaxAnalyzePasses {
		return fmt.Errorf("analyze budget exceeded")
	}
	r.stepsUsed++
	return nil
}

func (r *reasoningRun) recordArtifact(name string, data map[string]any) {
	if !r.runtime.config.StoreArtifactsInWorkingMemory || r.working == nil {
		return
	}
	r.working.AddRecord(memory.Record{
		Kind: reasoningKindArtifact,
		Name: name,
		Data: cloneMap(data),
	})
}

func (r *reasoningRun) recordDiagnostic(name string, data map[string]any) {
	if !r.runtime.config.EmitInternalDiagnostics || r.working == nil {
		return
	}
	r.working.AddRecord(memory.Record{
		Kind: reasoningKindDiagnostic,
		Name: name,
		Data: cloneMap(data),
	})
}

func (r *reasoningRun) interpretThink(artifact thinkArtifact, fallbackResponse string) (reasoningAction, error) {
	switch artifact.Decision {
	case "continue":
		return reasoningAction{
			kind: reasoningActionContinue,
			note: internalReasoningNote(artifact.Reason),
		}, nil
	case "call_tool":
		if isReservedReasoningToolName(artifact.CandidateToolName) {
			return reasoningAction{}, fmt.Errorf("reasoning suggested reserved tool %q", artifact.CandidateToolName)
		}
		return reasoningAction{
			kind:      reasoningActionCallTool,
			toolName:  artifact.CandidateToolName,
			toolInput: cloneMap(artifact.CandidateToolInput),
		}, nil
	case "respond":
		response := strings.TrimSpace(artifact.CandidateResponse)
		if response == "" {
			response = strings.TrimSpace(fallbackResponse)
		}
		if response == "" {
			return reasoningAction{}, fmt.Errorf("reasoning respond decision is missing candidate response")
		}
		return reasoningAction{
			kind:         reasoningActionRespond,
			toolResponse: response,
		}, nil
	case "validate":
		if r.runtime.config.Mode != ReasoningModeThinkAndAnalyze {
			return reasoningAction{}, fmt.Errorf("reasoning validate decision requires analyze")
		}
		return reasoningAction{}, nil
	case "fail":
		return reasoningAction{}, fmt.Errorf("reasoning failed: %s", artifact.Reason)
	default:
		return reasoningAction{}, fmt.Errorf("unsupported reasoning decision %q", artifact.Decision)
	}
}

func (r *reasoningRun) interpretAnalyze(artifact analyzeArtifact, fallbackResponse string) (reasoningAction, error) {
	switch artifact.Verdict {
	case "approved":
		switch artifact.RecommendedAction {
		case "respond":
			response := strings.TrimSpace(artifact.CandidateResponse)
			if response == "" {
				response = strings.TrimSpace(fallbackResponse)
			}
			if response == "" {
				return reasoningAction{}, fmt.Errorf("reasoning analyze respond is missing candidate response")
			}
			return reasoningAction{
				kind:         reasoningActionRespond,
				toolResponse: response,
			}, nil
		case "continue":
			return reasoningAction{
				kind: reasoningActionContinue,
				note: internalReasoningNote(artifact.Reason),
			}, nil
		default:
			return reasoningAction{}, fmt.Errorf("invalid approved analyze action %q", artifact.RecommendedAction)
		}
	case "needs_more_work":
		switch artifact.RecommendedAction {
		case "continue":
			return reasoningAction{
				kind: reasoningActionContinue,
				note: internalReasoningNote(artifact.Reason),
			}, nil
		case "call_tool":
			if isReservedReasoningToolName(artifact.CandidateToolName) {
				return reasoningAction{}, fmt.Errorf("reasoning analyze suggested reserved tool %q", artifact.CandidateToolName)
			}
			return reasoningAction{
				kind:      reasoningActionCallTool,
				toolName:  artifact.CandidateToolName,
				toolInput: cloneMap(artifact.CandidateToolInput),
			}, nil
		default:
			return reasoningAction{}, fmt.Errorf("invalid needs_more_work analyze action %q", artifact.RecommendedAction)
		}
	case "blocked":
		if artifact.RecommendedAction != "fail" {
			return reasoningAction{}, fmt.Errorf("blocked analyze verdict requires fail action, got %q", artifact.RecommendedAction)
		}
		return reasoningAction{}, nil
	default:
		return reasoningAction{}, fmt.Errorf("unsupported analyze verdict %q", artifact.Verdict)
	}
}

type reasoningToolkit struct {
	config ReasoningConfig
}

func newReasoningToolkit(cfg ReasoningConfig) reasoningToolkit {
	return reasoningToolkit{config: cfg}
}

func (t reasoningToolkit) Name() string {
	return reasoningToolkitName
}

func (t reasoningToolkit) Description() string {
	return "Internal planning and validation tools for runtime orchestration."
}

func (t reasoningToolkit) Namespace() string {
	return reasoningNamespace
}

func (t reasoningToolkit) Tools() []tool.Tool {
	tools := []tool.Tool{thinkTool{config: t.config}}
	if t.config.Mode == ReasoningModeThinkAndAnalyze {
		tools = append(tools, analyzeTool{})
	}
	return tools
}

type thinkTool struct {
	config ReasoningConfig
}

func (thinkTool) Name() string {
	return reasoningThinkName
}

func (thinkTool) Description() string {
	return "Plans the next runtime step from the current run snapshot."
}

func (thinkTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"objective":          {Type: "string"},
			"step_index":         {Type: "integer"},
			"max_steps":          {Type: "integer"},
			"messages":           {Type: "array"},
			"available_tools":    {Type: "array"},
			"constraints":        permissiveObjectSchema(),
			"last_model_output":  permissiveObjectSchema(),
			"last_tool_result":   permissiveObjectSchema(),
			"candidate_response": {Type: "string"},
			"metadata":           permissiveObjectSchema(),
		},
		Required: []string{
			"objective",
			"step_index",
			"max_steps",
			"messages",
			"available_tools",
			"constraints",
		},
	}
}

func (thinkTool) OutputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"decision":             {Type: "string", Enum: []string{"continue", "call_tool", "validate", "respond", "fail"}},
			"plan":                 {Type: "array"},
			"reason":               {Type: "string"},
			"candidate_tool_name":  {Type: "string"},
			"candidate_tool_input": permissiveObjectSchema(),
			"candidate_response":   {Type: "string"},
			"needs_validation":     {Type: "boolean"},
		},
		Required: []string{"decision", "plan", "reason"},
	}
}

func (t thinkTool) Call(_ context.Context, call tool.Call) (tool.Result, error) {
	candidateResponse, _ := call.Input["candidate_response"].(string)
	lastToolResult, _ := call.Input["last_tool_result"].(map[string]any)
	lastModelOutput, _ := call.Input["last_model_output"].(map[string]any)

	decision := "continue"
	reason := "continue the base agent loop"
	plan := []any{"continue agent loop"}
	needsValidation := false
	var candidateToolName string
	var candidateToolInput map[string]any

	if name, input := suggestedToolFromMap(lastModelOutput); name != "" {
		decision = "call_tool"
		reason = "execute the suggested public tool"
		plan = []any{"validate tool suggestion", "execute public tool"}
		candidateToolName = name
		candidateToolInput = input
	}

	if candidate := strings.TrimSpace(candidateResponse); candidate != "" {
		if t.config.RequireAnalyzeBeforeResponse && t.config.Mode == ReasoningModeThinkAndAnalyze {
			decision = "validate"
			reason = "validate the candidate response before final answer"
			plan = []any{"validate response", "finalize answer if approved"}
			needsValidation = true
		} else {
			decision = "respond"
			reason = "candidate response is ready for final delivery"
			plan = []any{"emit final answer"}
		}
	}

	if lastToolResult != nil && decision == "continue" {
		if t.config.AnalyzeAfterToolResult && t.config.Mode == ReasoningModeThinkAndAnalyze {
			decision = "validate"
			reason = "validate the latest public tool result before continuing"
			plan = []any{"validate tool result", "continue model loop"}
			needsValidation = true
		} else {
			reason = "continue the loop with the latest public tool result"
			plan = []any{"continue model loop", "use latest tool result"}
		}
	}

	if t.config.Mode == ReasoningModeThinkOnly && decision == "validate" {
		decision = "fail"
		reason = "validate is unavailable while analyze is disabled"
		plan = []any{"fail reasoning policy"}
		needsValidation = false
	}

	return tool.Result{
		Value: map[string]any{
			"decision":             decision,
			"plan":                 plan,
			"reason":               reason,
			"candidate_tool_name":  candidateToolName,
			"candidate_tool_input": cloneMap(candidateToolInput),
			"candidate_response":   strings.TrimSpace(candidateResponse),
			"needs_validation":     needsValidation,
		},
	}, nil
}

type analyzeTool struct{}

func (analyzeTool) Name() string {
	return reasoningAnalyzeName
}

func (analyzeTool) Description() string {
	return "Validates candidate runtime state before the next public action."
}

func (analyzeTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"objective":               {Type: "string"},
			"step_index":              {Type: "integer"},
			"max_steps":               {Type: "integer"},
			"candidate":               permissiveObjectSchema(),
			"checks":                  {Type: "array"},
			"messages":                {Type: "array"},
			"supporting_tool_results": {Type: "array"},
			"metadata":                permissiveObjectSchema(),
		},
		Required: []string{"objective", "step_index", "max_steps", "candidate", "checks"},
	}
}

func (analyzeTool) OutputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"verdict":              {Type: "string", Enum: []string{"approved", "needs_more_work", "blocked"}},
			"recommended_action":   {Type: "string", Enum: []string{"continue", "call_tool", "respond", "fail"}},
			"findings":             {Type: "array"},
			"reason":               {Type: "string"},
			"candidate_tool_name":  {Type: "string"},
			"candidate_tool_input": permissiveObjectSchema(),
			"candidate_response":   {Type: "string"},
		},
		Required: []string{"verdict", "recommended_action", "findings", "reason"},
	}
}

func (analyzeTool) Call(_ context.Context, call tool.Call) (tool.Result, error) {
	candidate, _ := call.Input["candidate"].(map[string]any)
	response := responseFromCandidate(candidate)
	findings := []any{}

	if name, input := suggestedToolFromMap(candidate); name != "" {
		findings = append(findings, map[string]any{
			"kind":      "tool_suggestion",
			"tool_name": name,
		})
		return tool.Result{
			Value: map[string]any{
				"verdict":              "needs_more_work",
				"recommended_action":   "call_tool",
				"findings":             findings,
				"reason":               "a public tool is still required",
				"candidate_tool_name":  name,
				"candidate_tool_input": cloneMap(input),
				"candidate_response":   "",
			},
		}, nil
	}

	if response != "" {
		if candidateResponseNeedsMoreWork(response) {
			findings = append(findings, map[string]any{
				"kind":   "candidate_response",
				"status": "needs_more_work",
			})
			return tool.Result{
				Value: map[string]any{
					"verdict":            "needs_more_work",
					"recommended_action": "continue",
					"findings":           findings,
					"reason":             "candidate response is still incomplete",
				},
			}, nil
		}

		findings = append(findings, map[string]any{
			"kind":   "candidate_response",
			"status": "approved",
		})
		return tool.Result{
			Value: map[string]any{
				"verdict":            "approved",
				"recommended_action": "respond",
				"findings":           findings,
				"reason":             "candidate response is grounded enough to answer",
				"candidate_response": response,
			},
		}, nil
	}

	if toolResult := nestedObject(candidate, "tool_output"); toolResult != nil {
		findings = append(findings, map[string]any{
			"kind":   "tool_result",
			"status": "approved",
		})
		return tool.Result{
			Value: map[string]any{
				"verdict":            "approved",
				"recommended_action": "continue",
				"findings":           findings,
				"reason":             "public tool result is ready for the next model step",
			},
		}, nil
	}

	findings = append(findings, map[string]any{
		"kind":   "candidate",
		"status": "blocked",
	})
	return tool.Result{
		Value: map[string]any{
			"verdict":            "blocked",
			"recommended_action": "fail",
			"findings":           findings,
			"reason":             "candidate state does not contain actionable data",
		},
	}, nil
}

func permissiveObjectSchema() tool.Schema {
	allow := true
	return tool.Schema{
		Type:                 "object",
		AdditionalProperties: &allow,
	}
}

func isReservedReasoningToolName(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, reasoningPrefix)
}

func internalReasoningNote(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	return internalReasoningPromptPrefix + " " + reason
}

func deriveObjective(definition agent.Definition, conversation []types.Message) string {
	for index := len(conversation) - 1; index >= 0; index-- {
		if conversation[index].Role != types.RoleUser {
			continue
		}
		if content := strings.TrimSpace(conversation[index].Content); content != "" {
			return content
		}
	}
	if content := strings.TrimSpace(definition.Instructions); content != "" {
		return content
	}
	return definition.Descriptor.Name
}

func messagesToMaps(messages []types.Message) []any {
	if len(messages) == 0 {
		return nil
	}
	out := make([]any, len(messages))
	for index, message := range messages {
		out[index] = messageToMap(message)
	}
	return out
}

func messageToMap(message types.Message) map[string]any {
	out := map[string]any{
		"role":    string(message.Role),
		"content": message.Content,
	}
	if message.Name != "" {
		out["name"] = message.Name
	}
	if message.ToolCallID != "" {
		out["tool_call_id"] = message.ToolCallID
	}
	if len(message.ToolCalls) > 0 {
		toolCalls := make([]any, len(message.ToolCalls))
		for index, call := range message.ToolCalls {
			toolCalls[index] = map[string]any{
				"id":    call.ID,
				"name":  call.Name,
				"input": cloneMap(call.Input),
			}
		}
		out["tool_calls"] = toolCalls
	}
	if len(message.Metadata) > 0 {
		out["metadata"] = metadataToMap(message.Metadata)
	}
	return out
}

func toolSpecsToMaps(registered map[string]tool.Tool) []any {
	specs := buildToolSpecs(registered)
	if len(specs) == 0 {
		return nil
	}
	out := make([]any, len(specs))
	for index, spec := range specs {
		out[index] = map[string]any{
			"name":        spec.Name,
			"description": spec.Description,
		}
	}
	return out
}

func constraintsToMap(definition agent.Definition, req agent.Request) map[string]any {
	out := map[string]any{
		"tool_choice": string(req.ToolChoice),
		"agent_id":    definition.Descriptor.ID,
		"run_id":      req.RunID,
	}
	if req.SessionID != "" {
		out["session_id"] = req.SessionID
	}
	if len(req.AllowedTools) > 0 {
		allowed := make([]any, len(req.AllowedTools))
		for index, name := range req.AllowedTools {
			allowed[index] = name
		}
		out["allowed_tools"] = allowed
	}
	return out
}

func metadataToMap(md types.Metadata) map[string]any {
	if len(md) == 0 {
		return nil
	}
	out := make(map[string]any, len(md))
	for key, value := range md {
		out[key] = value
	}
	return out
}

func modelResponseToMap(response agent.ModelResponse) map[string]any {
	out := map[string]any{
		"message": messageToMap(response.Message),
	}
	if len(response.ToolCalls) > 0 {
		toolCalls := make([]any, len(response.ToolCalls))
		for index, call := range response.ToolCalls {
			toolCalls[index] = map[string]any{
				"id":    call.ID,
				"name":  call.Name,
				"input": cloneMap(call.Input),
			}
		}
		out["tool_calls"] = toolCalls
	}
	if len(response.Metadata) > 0 {
		out["metadata"] = metadataToMap(response.Metadata)
	}
	return out
}

func lastToolCallToMap(records []agent.ToolCallRecord) map[string]any {
	if len(records) == 0 {
		return nil
	}
	return toolCallRecordToMap(records[len(records)-1])
}

func toolCallRecordToMap(record agent.ToolCallRecord) map[string]any {
	return map[string]any{
		"id":       record.ID,
		"name":     record.Name,
		"input":    cloneMap(record.Input),
		"output":   toolResultToMap(record.Output),
		"duration": record.Duration.String(),
	}
}

func toolResultToMap(result tool.Result) map[string]any {
	out := map[string]any{
		"value":   cloneValue(result.Value),
		"content": result.Content,
	}
	if len(result.Metadata) > 0 {
		out["metadata"] = metadataToMap(result.Metadata)
	}
	return out
}

func toolCallsToMaps(records []agent.ToolCallRecord) []any {
	if len(records) == 0 {
		return nil
	}
	out := make([]any, len(records))
	for index, record := range records {
		out[index] = toolCallRecordToMap(record)
	}
	return out
}

func thinkArtifactToMap(artifact thinkArtifact) map[string]any {
	return map[string]any{
		"decision":             artifact.Decision,
		"plan":                 cloneSlice(artifact.Plan),
		"reason":               artifact.Reason,
		"candidate_tool_name":  artifact.CandidateToolName,
		"candidate_tool_input": cloneMap(artifact.CandidateToolInput),
		"candidate_response":   artifact.CandidateResponse,
		"needs_validation":     artifact.NeedsValidation,
	}
}

func analyzeArtifactToMap(artifact analyzeArtifact) map[string]any {
	return map[string]any{
		"verdict":              artifact.Verdict,
		"recommended_action":   artifact.RecommendedAction,
		"findings":             cloneSlice(artifact.Findings),
		"reason":               artifact.Reason,
		"candidate_tool_name":  artifact.CandidateToolName,
		"candidate_tool_input": cloneMap(artifact.CandidateToolInput),
		"candidate_response":   artifact.CandidateResponse,
	}
}

func parseThinkArtifact(value any) (thinkArtifact, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return thinkArtifact{}, fmt.Errorf("reasoning think output must be object")
	}
	artifact := thinkArtifact{
		Decision:           stringFromMap(raw, "decision"),
		Plan:               sliceFromMap(raw, "plan"),
		Reason:             stringFromMap(raw, "reason"),
		CandidateToolName:  stringFromMap(raw, "candidate_tool_name"),
		CandidateToolInput: objectFromMap(raw, "candidate_tool_input"),
		CandidateResponse:  stringFromMap(raw, "candidate_response"),
		NeedsValidation:    boolFromMap(raw, "needs_validation"),
	}
	switch artifact.Decision {
	case "continue", "validate", "respond", "fail":
	case "call_tool":
		if artifact.CandidateToolName == "" || artifact.CandidateToolInput == nil {
			return thinkArtifact{}, fmt.Errorf("reasoning think call_tool requires tool name and input")
		}
	default:
		return thinkArtifact{}, fmt.Errorf("unsupported reasoning think decision %q", artifact.Decision)
	}
	if artifact.Decision == "respond" && strings.TrimSpace(artifact.CandidateResponse) == "" {
		return thinkArtifact{}, fmt.Errorf("reasoning think respond decision requires candidate response")
	}
	if strings.TrimSpace(artifact.Reason) == "" {
		return thinkArtifact{}, fmt.Errorf("reasoning think reason is required")
	}
	return artifact, nil
}

func parseAnalyzeArtifact(value any) (analyzeArtifact, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return analyzeArtifact{}, fmt.Errorf("reasoning analyze output must be object")
	}
	artifact := analyzeArtifact{
		Verdict:            stringFromMap(raw, "verdict"),
		RecommendedAction:  stringFromMap(raw, "recommended_action"),
		Findings:           sliceFromMap(raw, "findings"),
		Reason:             stringFromMap(raw, "reason"),
		CandidateToolName:  stringFromMap(raw, "candidate_tool_name"),
		CandidateToolInput: objectFromMap(raw, "candidate_tool_input"),
		CandidateResponse:  stringFromMap(raw, "candidate_response"),
	}

	switch artifact.Verdict {
	case "approved":
		if artifact.RecommendedAction != "respond" && artifact.RecommendedAction != "continue" {
			return analyzeArtifact{}, fmt.Errorf("approved analyze verdict requires respond or continue")
		}
	case "needs_more_work":
		if artifact.RecommendedAction != "continue" && artifact.RecommendedAction != "call_tool" {
			return analyzeArtifact{}, fmt.Errorf("needs_more_work analyze verdict requires continue or call_tool")
		}
	case "blocked":
		if artifact.RecommendedAction != "fail" {
			return analyzeArtifact{}, fmt.Errorf("blocked analyze verdict requires fail")
		}
	default:
		return analyzeArtifact{}, fmt.Errorf("unsupported analyze verdict %q", artifact.Verdict)
	}
	if artifact.RecommendedAction == "call_tool" && (artifact.CandidateToolName == "" || artifact.CandidateToolInput == nil) {
		return analyzeArtifact{}, fmt.Errorf("analyze call_tool requires tool name and input")
	}
	if artifact.RecommendedAction == "respond" && strings.TrimSpace(artifact.CandidateResponse) == "" {
		return analyzeArtifact{}, fmt.Errorf("analyze respond requires candidate response")
	}
	if strings.TrimSpace(artifact.Reason) == "" {
		return analyzeArtifact{}, fmt.Errorf("analyze reason is required")
	}
	return artifact, nil
}

func stringFromMap(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func sliceFromMap(raw map[string]any, key string) []any {
	value, _ := raw[key].([]any)
	return cloneSlice(value)
}

func objectFromMap(raw map[string]any, key string) map[string]any {
	value, _ := raw[key].(map[string]any)
	return cloneMap(value)
}

func boolFromMap(raw map[string]any, key string) bool {
	value, _ := raw[key].(bool)
	return value
}

func nestedObject(raw map[string]any, key string) map[string]any {
	if raw == nil {
		return nil
	}
	value, _ := raw[key].(map[string]any)
	return value
}

func suggestedToolFromMap(raw map[string]any) (string, map[string]any) {
	if raw == nil {
		return "", nil
	}
	name := stringFromMap(raw, "candidate_tool_name")
	input := objectFromMap(raw, "candidate_tool_input")
	if name != "" && input != nil {
		return name, input
	}

	name = stringFromMap(raw, "suggested_tool_name")
	input = objectFromMap(raw, "suggested_tool_input")
	if name != "" && input != nil {
		return name, input
	}

	toolCalls, _ := raw["tool_calls"].([]any)
	if len(toolCalls) == 0 {
		return "", nil
	}
	first, _ := toolCalls[0].(map[string]any)
	if first == nil {
		return "", nil
	}
	name = stringFromMap(first, "name")
	input = objectFromMap(first, "input")
	if name == "" || input == nil {
		return "", nil
	}
	return name, input
}

func responseFromCandidate(candidate map[string]any) string {
	if candidate == nil {
		return ""
	}
	if response := stringFromMap(candidate, "candidate_response"); response != "" {
		return strings.TrimSpace(response)
	}
	if response := stringFromMap(candidate, "response"); response != "" {
		return strings.TrimSpace(response)
	}
	if message := nestedObject(candidate, "message"); message != nil {
		return strings.TrimSpace(stringFromMap(message, "content"))
	}
	return ""
}

func candidateResponseNeedsMoreWork(response string) bool {
	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" {
		return true
	}
	markers := []string{
		"need more information",
		"insufficient information",
		"todo",
		"placeholder",
	}
	for _, marker := range markers {
		if strings.Contains(response, marker) {
			return true
		}
	}
	return false
}

func filterPersistedRecords(records []memory.Record) []memory.Record {
	if len(records) == 0 {
		return nil
	}
	filtered := make([]memory.Record, 0, len(records))
	for _, record := range records {
		if record.Kind == reasoningKindArtifact || record.Kind == reasoningKindDiagnostic {
			continue
		}
		filtered = append(filtered, memory.Record{
			Kind: record.Kind,
			Name: record.Name,
			Data: cloneMap(record.Data),
		})
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
