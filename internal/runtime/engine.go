package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// NewEngine returns the default in-process execution engine used by pkg/app.
func NewEngine() agent.Engine {
	return &engine{}
}

type engine struct{}

func (e *engine) Run(ctx context.Context, a *agent.Agent, req agent.Request) (agent.Response, error) {
	definition := a.Definition()
	emitter := newEventEmitter(definition, req, nil)

	response, err := e.execute(ctx, definition, req, emitter)
	if err != nil {
		return agent.Response{}, err
	}
	return response, nil
}

func (e *engine) Stream(ctx context.Context, a *agent.Agent, req agent.Request) (agent.Stream, error) {
	definition := a.Definition()
	runCtx, cancel := context.WithCancelCause(ctx)
	sink := newStreamSink(cancel)
	emitter := newEventEmitter(definition, req, sink)

	go func() {
		defer sink.finish()

		_, err := e.execute(runCtx, definition, req, emitter)
		if errors.Is(err, agent.ErrStreamAborted) {
			sink.setPostDrainErr(agent.ErrStreamAborted)
		}
	}()

	return sink, nil
}

func (e *engine) execute(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
) (agent.Response, error) {
	if err := checkCanceled(ctx, "run", definition.Descriptor.ID, req.RunID); err != nil {
		return agent.Response{}, emitter.fail(ctx, err)
	}

	emitter.emit(ctx, agent.EventAgentStarted, false, func(event *agent.Event) {})

	messages, decisions, err := applyInputGuardrails(ctx, definition, req, emitter)
	if err != nil {
		return agent.Response{}, emitter.fail(ctx, err)
	}

	memorySnapshot := memory.Snapshot{}
	if definition.Memory != nil {
		loaded, loadErr := definition.Memory.Load(ctx, req.SessionID)
		if loadErr != nil {
			return agent.Response{}, emitter.fail(ctx, classifyError("memory.load", definition.Descriptor.ID, req.RunID, loadErr, agent.ErrorKindMemory))
		}
		memorySnapshot = cloneMemorySnapshot(loaded)
	}

	working, err := definition.WorkingMemory.NewRunState(ctx, definition.Descriptor.ID, req.RunID)
	if err != nil {
		return agent.Response{}, emitter.fail(ctx, classifyError("working_memory.new", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal))
	}

	for _, message := range memorySnapshot.Messages {
		working.AddMessage(message)
	}
	for _, message := range messages {
		working.AddMessage(message)
	}

	effectiveTools := filterTools(definition.Tools, req)
	conversation := append(types.CloneMessages(memorySnapshot.Messages), types.CloneMessages(messages)...)
	usage := types.Usage{}
	toolCalls := make([]agent.ToolCallRecord, 0)
	toolUsed := false

	for step := 0; step < req.MaxSteps; step++ {
		if err := checkCanceled(ctx, "model.generate", definition.Descriptor.ID, req.RunID); err != nil {
			return agent.Response{}, emitter.fail(ctx, err)
		}

		modelRequest := agent.ModelRequest{
			AgentID:      definition.Descriptor.ID,
			RunID:        req.RunID,
			SessionID:    req.SessionID,
			Instructions: definition.Instructions,
			Messages:     types.CloneMessages(conversation),
			Memory:       cloneMemorySnapshot(memorySnapshot),
			Metadata:     types.CloneMetadata(req.Metadata),
			MaxSteps:     req.MaxSteps,
			ToolChoice:   req.ToolChoice,
			AllowedTools: append([]string(nil), req.AllowedTools...),
			Tools:        buildToolSpecs(effectiveTools),
		}

		modelResponse, modelErr := definition.Model.Generate(ctx, modelRequest)
		if modelErr != nil {
			return agent.Response{}, emitter.fail(ctx, classifyError("model.generate", definition.Descriptor.ID, req.RunID, modelErr, agent.ErrorKindModel))
		}

		usage = usage.Add(modelResponse.Usage)

		if len(modelResponse.ToolCalls) == 0 {
			if req.ToolChoice == agent.ToolChoiceRequired && !toolUsed {
				requiredErr := fmt.Errorf("tool call required but none executed")
				return agent.Response{}, emitter.fail(ctx, classifyError("tool.required", definition.Descriptor.ID, req.RunID, requiredErr, agent.ErrorKindTool))
			}

			finalMessage := types.CloneMessage(modelResponse.Message)
			if finalMessage.Role == "" {
				finalMessage.Role = types.RoleAssistant
			}

			finalMessage, outputDecisions, guardrailErr := applyOutputGuardrails(ctx, definition, req, emitter, finalMessage)
			if guardrailErr != nil {
				return agent.Response{}, emitter.fail(ctx, guardrailErr)
			}
			decisions = append(decisions, outputDecisions...)

			working.AddMessage(finalMessage)
			workingSnapshot := working.Snapshot()

			responseMetadata := types.MergeMetadata(req.Metadata, modelResponse.Metadata)
			if definition.Memory != nil {
				saveErr := definition.Memory.Save(ctx, req.SessionID, memory.Delta{
					Messages: types.CloneMessages(workingSnapshot.Messages),
					Records:  cloneRecords(workingSnapshot.Records),
					Response: cloneMessagePointer(finalMessage),
					Metadata: types.CloneMetadata(responseMetadata),
				})
				if saveErr != nil {
					return agent.Response{}, emitter.fail(ctx, classifyError("memory.save", definition.Descriptor.ID, req.RunID, saveErr, agent.ErrorKindMemory))
				}
			}

			response := agent.Response{
				RunID:              req.RunID,
				AgentID:            definition.Descriptor.ID,
				SessionID:          req.SessionID,
				Message:            finalMessage,
				Usage:              usage,
				ToolCalls:          cloneToolCallRecords(toolCalls),
				GuardrailDecisions: cloneGuardrailDecisions(decisions),
				Metadata:           responseMetadata,
			}

			emitter.emit(ctx, agent.EventAgentCompleted, true, func(event *agent.Event) {
				cloned := response
				event.Response = &cloned
				event.Metadata = types.CloneMetadata(response.Metadata)
			})

			return response, nil
		}

		if req.ToolChoice == agent.ToolChoiceNone {
			toolErr := fmt.Errorf("tool call requested while tools are disabled")
			return agent.Response{}, emitter.fail(ctx, classifyError("tool.disabled", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool))
		}

		modelToolCalls := cloneModelToolCalls(modelResponse.ToolCalls)
		conversation = append(conversation, types.Message{
			Role:      types.RoleAssistant,
			ToolCalls: toTypesToolCalls(modelToolCalls),
		})
		working.AddMessage(types.Message{
			Role:      types.RoleAssistant,
			ToolCalls: toTypesToolCalls(modelToolCalls),
		})

		for _, modelToolCall := range modelToolCalls {
			if err := checkCanceled(ctx, "tool.call", definition.Descriptor.ID, req.RunID); err != nil {
				return agent.Response{}, emitter.fail(ctx, err)
			}

			selectedTool, ok := effectiveTools[modelToolCall.Name]
			if !ok {
				toolErr := fmt.Errorf("tool %q is not available for this run", modelToolCall.Name)
				return agent.Response{}, emitter.fail(ctx, classifyError("tool.resolve", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool))
			}

			toolUsed = true
			callRecord := agent.ToolCallRecord{
				ID:    modelToolCall.ID,
				Name:  modelToolCall.Name,
				Input: cloneMap(modelToolCall.Input),
			}

			emitter.emit(ctx, agent.EventToolCall, false, func(event *agent.Event) {
				event.ToolCall = &agent.ToolCallEvent{
					Call:   cloneToolCallRecord(callRecord),
					Status: agent.ToolCallStarted,
				}
			})

			startedAt := time.Now()
			result, toolErr := selectedTool.Call(ctx, tool.Call{
				ID:        modelToolCall.ID,
				RunID:     req.RunID,
				SessionID: req.SessionID,
				Input:     cloneMap(modelToolCall.Input),
				Metadata:  types.CloneMetadata(req.Metadata),
			})
			callRecord.Duration = time.Since(startedAt)

			if toolErr != nil {
				emitter.emit(ctx, agent.EventToolResult, false, func(event *agent.Event) {
					event.ToolCall = &agent.ToolCallEvent{
						Call:   cloneToolCallRecord(callRecord),
						Status: agent.ToolCallFailed,
					}
					event.Err = classifyError("tool.call", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool)
				})

				return agent.Response{}, emitter.fail(ctx, classifyError("tool.call", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool))
			}

			callRecord.Output = cloneToolResult(result)
			toolCalls = append(toolCalls, callRecord)

			emitter.emit(ctx, agent.EventToolResult, false, func(event *agent.Event) {
				event.ToolCall = &agent.ToolCallEvent{
					Call:   cloneToolCallRecord(callRecord),
					Status: agent.ToolCallSucceeded,
				}
			})

			working.AddRecord(memory.Record{
				Kind: "tool_call",
				Name: modelToolCall.Name,
				Data: map[string]any{
					"call_id": modelToolCall.ID,
					"input":   cloneMap(modelToolCall.Input),
					"output":  cloneToolResult(result),
				},
			})

			toolMessage := types.Message{
				Role:       types.RoleTool,
				Name:       modelToolCall.Name,
				ToolCallID: modelToolCall.ID,
				Content:    toolResultContent(result),
				Metadata:   types.CloneMetadata(result.Metadata),
			}
			conversation = append(conversation, toolMessage)
			working.AddMessage(toolMessage)
		}

		if step == req.MaxSteps-1 {
			return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
		}
	}

	return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
}

var errMaxSteps = errors.New("max steps exceeded")

type eventEmitter struct {
	definition agent.Definition
	req        agent.Request
	stream     *streamSink
	sequence   int64
}

func newEventEmitter(definition agent.Definition, req agent.Request, stream *streamSink) *eventEmitter {
	return &eventEmitter{
		definition: definition,
		req:        req,
		stream:     stream,
	}
}

func (e *eventEmitter) emit(
	ctx context.Context,
	eventType agent.EventType,
	terminal bool,
	mutate func(*agent.Event),
) {
	event := agent.Event{
		Sequence:  e.sequence + 1,
		Type:      eventType,
		RunID:     e.req.RunID,
		AgentID:   e.definition.Descriptor.ID,
		SessionID: e.req.SessionID,
		Time:      time.Now(),
		Metadata:  types.CloneMetadata(e.req.Metadata),
	}
	e.sequence++

	if mutate != nil {
		mutate(&event)
	}

	deliverToStream := true
	if terminal && errors.Is(event.Err, agent.ErrStreamAborted) {
		deliverToStream = false
	}

	if e.stream != nil && deliverToStream {
		if terminal {
			e.stream.markTerminalSent()
		}
		select {
		case e.stream.events <- event:
		case <-ctx.Done():
		}
	}

	for _, hook := range e.definition.Hooks {
		if hook == nil {
			continue
		}
		func(h agent.Hook) {
			defer func() {
				_ = recover()
			}()
			h.OnEvent(ctx, event)
		}(hook)
	}
}

func (e *eventEmitter) fail(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, agent.ErrStreamAborted), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		e.emit(ctx, agent.EventAgentCanceled, true, func(event *agent.Event) {
			event.Err = err
		})
	default:
		e.emit(ctx, agent.EventAgentFailed, true, func(event *agent.Event) {
			event.Err = err
		})
	}

	return err
}

type streamSink struct {
	events chan agent.Event
	cancel context.CancelCauseFunc

	mu           sync.Mutex
	postDrainErr error
	terminalSent bool
	done         bool
	closeOnce    sync.Once
}

func newStreamSink(cancel context.CancelCauseFunc) *streamSink {
	return &streamSink{
		events: make(chan agent.Event, 32),
		cancel: cancel,
	}
}

func (s *streamSink) Recv() (agent.Event, error) {
	event, ok := <-s.events
	if ok {
		return event, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.postDrainErr != nil {
		err := s.postDrainErr
		s.postDrainErr = nil
		return agent.Event{}, err
	}

	return agent.Event{}, io.EOF
}

func (s *streamSink) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if !s.done && !s.terminalSent && s.postDrainErr == nil {
			s.postDrainErr = agent.ErrStreamAborted
		}
		s.mu.Unlock()

		s.cancel(agent.ErrStreamAborted)
	})
	return nil
}

func (s *streamSink) markTerminalSent() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.terminalSent = true
}

func (s *streamSink) setPostDrainErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.terminalSent || s.done || s.postDrainErr != nil {
		return
	}
	s.postDrainErr = err
}

func (s *streamSink) finish() {
	s.mu.Lock()
	s.done = true
	s.mu.Unlock()

	close(s.events)
}

func applyInputGuardrails(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
) ([]types.Message, []agent.GuardrailDecision, error) {
	messages := types.CloneMessages(req.Messages)
	decisions := make([]agent.GuardrailDecision, 0, len(definition.InputGuardrails))

	for _, rule := range definition.InputGuardrails {
		if rule == nil {
			continue
		}

		decision, err := rule.CheckInput(ctx, guardrail.InputRequest{
			AgentID:   definition.Descriptor.ID,
			AgentName: definition.Descriptor.Name,
			RunID:     req.RunID,
			SessionID: req.SessionID,
			Messages:  types.CloneMessages(messages),
			Metadata:  types.CloneMetadata(req.Metadata),
		})
		if err != nil {
			return nil, nil, classifyError("guardrail.input", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}

		normalized := normalizeGuardrailDecision(agent.GuardrailPhaseInput, decision, rule)
		decisions = append(decisions, normalized)
		emitter.emit(ctx, agent.EventGuardrail, false, func(event *agent.Event) {
			event.Guardrail = &agent.GuardrailEvent{Decision: normalized}
		})

		switch decision.Action {
		case guardrail.ActionAllow:
		case guardrail.ActionTransform:
			if decision.Messages != nil {
				messages = types.CloneMessages(decision.Messages)
			}
		case guardrail.ActionBlock:
			blockErr := &agent.Error{
				Kind:    agent.ErrorKindGuardrailBlocked,
				Op:      "guardrail.input",
				AgentID: definition.Descriptor.ID,
				RunID:   req.RunID,
				Cause:   fmt.Errorf("%w: %s", agent.ErrGuardrailBlocked, normalized.Reason),
			}
			return nil, decisions, blockErr
		}
	}

	return messages, decisions, nil
}

func applyOutputGuardrails(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	message types.Message,
) (types.Message, []agent.GuardrailDecision, error) {
	out := types.CloneMessage(message)
	decisions := make([]agent.GuardrailDecision, 0, len(definition.OutputGuardrails))

	for _, rule := range definition.OutputGuardrails {
		if rule == nil {
			continue
		}

		decision, err := rule.CheckOutput(ctx, guardrail.OutputRequest{
			AgentID:   definition.Descriptor.ID,
			AgentName: definition.Descriptor.Name,
			RunID:     req.RunID,
			SessionID: req.SessionID,
			Message:   types.CloneMessage(out),
			Metadata:  types.CloneMetadata(req.Metadata),
		})
		if err != nil {
			return types.Message{}, nil, classifyError("guardrail.output", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}

		normalized := normalizeGuardrailDecision(agent.GuardrailPhaseOutput, decision, rule)
		decisions = append(decisions, normalized)
		emitter.emit(ctx, agent.EventGuardrail, false, func(event *agent.Event) {
			event.Guardrail = &agent.GuardrailEvent{Decision: normalized}
		})

		switch decision.Action {
		case guardrail.ActionAllow:
		case guardrail.ActionTransform:
			if decision.Message != nil {
				out = types.CloneMessage(*decision.Message)
			}
		case guardrail.ActionBlock:
			blockErr := &agent.Error{
				Kind:    agent.ErrorKindGuardrailBlocked,
				Op:      "guardrail.output",
				AgentID: definition.Descriptor.ID,
				RunID:   req.RunID,
				Cause:   fmt.Errorf("%w: %s", agent.ErrGuardrailBlocked, normalized.Reason),
			}
			return types.Message{}, decisions, blockErr
		}
	}

	return out, decisions, nil
}

func normalizeGuardrailDecision(
	phase agent.GuardrailPhase,
	decision guardrail.Decision,
	rule any,
) agent.GuardrailDecision {
	name := decision.Name
	if name == "" {
		name = fmt.Sprintf("%T", rule)
	}

	return agent.GuardrailDecision{
		Phase:    phase,
		Name:     name,
		Action:   decision.Action,
		Reason:   decision.Reason,
		Metadata: types.CloneMetadata(decision.Metadata),
	}
}

func classifyError(op, agentID, runID string, err error, kind agent.ErrorKind) error {
	switch {
	case errors.Is(err, agent.ErrStreamAborted):
		return &agent.Error{
			Kind:    agent.ErrorKindStreamAborted,
			Op:      op,
			AgentID: agentID,
			RunID:   runID,
			Cause:   agent.ErrStreamAborted,
		}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return &agent.Error{
			Kind:    agent.ErrorKindCanceled,
			Op:      op,
			AgentID: agentID,
			RunID:   runID,
			Cause:   err,
		}
	case errors.Is(err, agent.ErrGuardrailBlocked):
		return &agent.Error{
			Kind:    agent.ErrorKindGuardrailBlocked,
			Op:      op,
			AgentID: agentID,
			RunID:   runID,
			Cause:   err,
		}
	case errors.Is(err, errMaxSteps), errors.Is(err, agent.ErrMaxStepsExceeded):
		return &agent.Error{
			Kind:    agent.ErrorKindMaxSteps,
			Op:      op,
			AgentID: agentID,
			RunID:   runID,
			Cause:   agent.ErrMaxStepsExceeded,
		}
	default:
		return &agent.Error{
			Kind:    kind,
			Op:      op,
			AgentID: agentID,
			RunID:   runID,
			Cause:   err,
		}
	}
}

func checkCanceled(ctx context.Context, op, agentID, runID string) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		cause := context.Cause(ctx)
		if cause == nil {
			cause = err
		}
		return classifyError(op, agentID, runID, cause, agent.ErrorKindCanceled)
	}
	return nil
}

func filterTools(registered []tool.Tool, req agent.Request) map[string]tool.Tool {
	if req.ToolChoice == agent.ToolChoiceNone {
		return nil
	}

	allowed := make(map[string]struct{}, len(req.AllowedTools))
	for _, name := range req.AllowedTools {
		allowed[name] = struct{}{}
	}

	out := make(map[string]tool.Tool, len(registered))
	for _, registeredTool := range registered {
		if len(allowed) > 0 {
			if _, ok := allowed[registeredTool.Name()]; !ok {
				continue
			}
		}
		out[registeredTool.Name()] = registeredTool
	}
	return out
}

func buildToolSpecs(registered map[string]tool.Tool) []agent.ToolSpec {
	if len(registered) == 0 {
		return nil
	}

	names := make([]string, 0, len(registered))
	for name := range registered {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]agent.ToolSpec, 0, len(registered))
	for _, name := range names {
		registeredTool := registered[name]
		out = append(out, agent.ToolSpec{
			Name:        registeredTool.Name(),
			Description: registeredTool.Description(),
		})
	}
	return out
}

func cloneMemorySnapshot(snapshot memory.Snapshot) memory.Snapshot {
	return memory.Snapshot{
		Messages: types.CloneMessages(snapshot.Messages),
		Records:  cloneRecords(snapshot.Records),
		Metadata: types.CloneMetadata(snapshot.Metadata),
	}
}

func cloneRecords(records []memory.Record) []memory.Record {
	if len(records) == 0 {
		return nil
	}

	out := make([]memory.Record, len(records))
	for index, record := range records {
		out[index] = memory.Record{
			Kind: record.Kind,
			Name: record.Name,
			Data: cloneMap(record.Data),
		}
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneMessagePointer(message types.Message) *types.Message {
	out := types.CloneMessage(message)
	return &out
}

func cloneToolCallRecord(record agent.ToolCallRecord) agent.ToolCallRecord {
	return agent.ToolCallRecord{
		ID:       record.ID,
		Name:     record.Name,
		Input:    cloneMap(record.Input),
		Output:   cloneToolResult(record.Output),
		Duration: record.Duration,
	}
}

func cloneToolCallRecords(records []agent.ToolCallRecord) []agent.ToolCallRecord {
	if len(records) == 0 {
		return nil
	}

	out := make([]agent.ToolCallRecord, len(records))
	for index, record := range records {
		out[index] = cloneToolCallRecord(record)
	}
	return out
}

func cloneGuardrailDecisions(in []agent.GuardrailDecision) []agent.GuardrailDecision {
	if len(in) == 0 {
		return nil
	}

	out := make([]agent.GuardrailDecision, len(in))
	for index, decision := range in {
		out[index] = agent.GuardrailDecision{
			Phase:    decision.Phase,
			Name:     decision.Name,
			Action:   decision.Action,
			Reason:   decision.Reason,
			Metadata: types.CloneMetadata(decision.Metadata),
		}
	}
	return out
}

func cloneToolResult(result tool.Result) tool.Result {
	return tool.Result{
		Content:  result.Content,
		Data:     cloneMap(result.Data),
		Metadata: types.CloneMetadata(result.Metadata),
	}
}

func toolResultContent(result tool.Result) string {
	if result.Content != "" {
		return result.Content
	}
	if len(result.Data) == 0 {
		return ""
	}

	raw, err := json.Marshal(result.Data)
	if err != nil {
		return fmt.Sprintf("%v", result.Data)
	}
	return string(raw)
}

func cloneModelToolCalls(calls []agent.ModelToolCall) []agent.ModelToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]agent.ModelToolCall, len(calls))
	for index, call := range calls {
		out[index] = agent.ModelToolCall{
			ID:    call.ID,
			Name:  call.Name,
			Input: cloneMap(call.Input),
		}
	}
	return out
}

func toTypesToolCalls(calls []agent.ModelToolCall) []types.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]types.ToolCall, len(calls))
	for index, call := range calls {
		out[index] = types.ToolCall{
			ID:    call.ID,
			Name:  call.Name,
			Input: cloneMap(call.Input),
		}
	}
	return out
}
