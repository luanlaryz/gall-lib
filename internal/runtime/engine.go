package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// NewEngine returns the default in-process execution engine used by pkg/app.
func NewEngine(opts ...Option) agent.Engine {
	resolved := defaultEngineOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&resolved)
	}

	reasoning, err := newReasoningRuntime(resolved.reasoning)
	return &engine{
		reasoning:    reasoning,
		reasoningErr: err,
	}
}

type engine struct {
	reasoning    *reasoningRuntime
	reasoningErr error
}

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
	if e.reasoningErr != nil {
		return agent.Response{}, classifyError("reasoning.init", definition.Descriptor.ID, req.RunID, e.reasoningErr, agent.ErrorKindInternal)
	}
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
	reasoning := newReasoningRun(e.reasoning, working)

	for step := 0; step < req.MaxSteps; step++ {
		modelOp := "model.generate"
		if emitter.stream != nil {
			modelOp = "model.stream"
		}
		if err := checkCanceled(ctx, modelOp, definition.Descriptor.ID, req.RunID); err != nil {
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

		modelResponse, streamDecisions, modelErr := e.collectModelResponse(ctx, definition, req, emitter, modelRequest)
		if modelErr != nil {
			var agentErr *agent.Error
			if errors.As(modelErr, &agentErr) {
				return agent.Response{}, emitter.fail(ctx, modelErr)
			}
			return agent.Response{}, emitter.fail(ctx, classifyError(modelOp, definition.Descriptor.ID, req.RunID, modelErr, agent.ErrorKindModel))
		}
		decisions = append(decisions, streamDecisions...)

		usage = usage.Add(modelResponse.Usage)

		if len(modelResponse.ToolCalls) == 0 {
			finalMessage := types.CloneMessage(modelResponse.Message)
			if finalMessage.Role == "" {
				finalMessage.Role = types.RoleAssistant
			}

			if reasoning != nil {
				reasoningAction, reasoningErr := reasoning.beforeFinalResponse(
					ctx,
					definition,
					req,
					conversation,
					effectiveTools,
					modelResponse,
					toolCalls,
					finalMessage,
					step+1,
				)
				if reasoningErr != nil {
					return agent.Response{}, emitter.fail(ctx, classifyError("reasoning.final", definition.Descriptor.ID, req.RunID, reasoningErr, agent.ErrorKindInternal))
				}

				switch reasoningAction.kind {
				case reasoningActionContinue:
					conversation = appendInternalReasoningNote(conversation, reasoningAction.note)
					if step == req.MaxSteps-1 {
						return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
					}
					continue
				case reasoningActionCallTool:
					if err := e.executeSuggestedToolCall(
						ctx,
						definition,
						req,
						emitter,
						effectiveTools,
						&conversation,
						working,
						&toolCalls,
						&toolUsed,
						step,
						reasoningAction,
					); err != nil {
						return agent.Response{}, emitter.fail(ctx, err)
					}
					if step == req.MaxSteps-1 {
						return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
					}
					continue
				case reasoningActionRespond:
					finalMessage.Content = reasoningAction.toolResponse
				}
			}

			if req.ToolChoice == agent.ToolChoiceRequired && !toolUsed {
				requiredErr := fmt.Errorf("tool call required but none executed")
				return agent.Response{}, emitter.fail(ctx, classifyError("tool.required", definition.Descriptor.ID, req.RunID, requiredErr, agent.ErrorKindTool))
			}

			finalMessage, outputDecisions, guardrailErr := applyOutputGuardrails(ctx, definition, req, emitter, finalMessage)
			if guardrailErr != nil {
				return agent.Response{}, emitter.fail(ctx, guardrailErr)
			}
			decisions = append(decisions, outputDecisions...)

			working.AddMessage(finalMessage)
			workingSnapshot := working.Snapshot()

			responseMetadata := preserveCorrelationMetadata(types.MergeMetadata(req.Metadata, modelResponse.Metadata), req.Metadata)
			if definition.Memory != nil {
				saveErr := definition.Memory.Save(ctx, req.SessionID, memory.Delta{
					Messages: types.CloneMessages(workingSnapshot.Messages),
					Records:  filterPersistedRecords(workingSnapshot.Records),
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

		for index, modelToolCall := range modelToolCalls {
			if err := checkCanceled(ctx, "tool.call", definition.Descriptor.ID, req.RunID); err != nil {
				return agent.Response{}, emitter.fail(ctx, err)
			}
			callRecord, invokeErr := e.invokePublicToolCall(
				ctx,
				definition,
				req,
				emitter,
				effectiveTools,
				modelToolCall,
				step,
				index,
				&toolCalls,
				&toolUsed,
			)
			if invokeErr != nil {
				return agent.Response{}, emitter.fail(ctx, invokeErr)
			}

			working.AddRecord(memory.Record{
				Kind: "tool_call",
				Name: callRecord.Name,
				Data: map[string]any{
					"call_id": callRecord.ID,
					"input":   cloneMap(callRecord.Input),
					"output":  cloneToolResult(callRecord.Output),
				},
			})

			toolMessage := types.Message{
				Role:       types.RoleTool,
				Name:       callRecord.Name,
				ToolCallID: callRecord.ID,
				Content:    toolResultContent(callRecord.Output),
				Metadata:   types.CloneMetadata(callRecord.Output.Metadata),
			}
			conversation = append(conversation, toolMessage)
			working.AddMessage(toolMessage)

			if reasoning != nil {
				reasoningAction, reasoningErr := reasoning.afterToolResult(
					ctx,
					definition,
					req,
					conversation,
					effectiveTools,
					step+1,
					callRecord,
				)
				if reasoningErr != nil {
					return agent.Response{}, emitter.fail(ctx, classifyError("reasoning.tool_result", definition.Descriptor.ID, req.RunID, reasoningErr, agent.ErrorKindInternal))
				}
				if reasoningAction.kind == reasoningActionCallTool {
					if err := e.executeSuggestedToolCall(
						ctx,
						definition,
						req,
						emitter,
						effectiveTools,
						&conversation,
						working,
						&toolCalls,
						&toolUsed,
						step,
						reasoningAction,
					); err != nil {
						return agent.Response{}, emitter.fail(ctx, err)
					}
				}
				conversation = appendInternalReasoningNote(conversation, reasoningAction.note)
			}
		}

		if step == req.MaxSteps-1 {
			return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
		}
	}

	return agent.Response{}, emitter.fail(ctx, classifyError("run.max_steps", definition.Descriptor.ID, req.RunID, errMaxSteps, agent.ErrorKindMaxSteps))
}

func (e *engine) collectModelResponse(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	modelRequest agent.ModelRequest,
) (agent.ModelResponse, []agent.GuardrailDecision, error) {
	if emitter.stream == nil {
		modelResponse, err := definition.Model.Generate(ctx, modelRequest)
		return modelResponse, nil, err
	}

	modelResponse, decisions, streamed, err := e.collectStreamingModelResponse(ctx, definition, req, emitter, modelRequest)
	if err != nil {
		return agent.ModelResponse{}, nil, err
	}
	if streamed {
		return modelResponse, decisions, nil
	}

	modelResponse, err = definition.Model.Generate(ctx, modelRequest)
	return modelResponse, nil, err
}

func (e *engine) collectStreamingModelResponse(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	modelRequest agent.ModelRequest,
) (agent.ModelResponse, []agent.GuardrailDecision, bool, error) {
	modelStream, err := definition.Model.Stream(ctx, modelRequest)
	if err != nil {
		return agent.ModelResponse{}, nil, false, err
	}
	defer func() {
		_ = modelStream.Close()
	}()

	var (
		response            agent.ModelResponse
		decisions           []agent.GuardrailDecision
		sawEvent            bool
		sawDelta            bool
		sawStreamAdjustment bool
		bufferedContent     string
		chunkIndex          int64
	)

	for {
		event, recvErr := modelStream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			return agent.ModelResponse{}, decisions, sawEvent, recvErr
		}

		sawEvent = true
		response.Usage = response.Usage.Add(event.Usage)

		if event.Delta != nil {
			sawDelta = true
			chunkIndex++

			delta, streamDecisions, outcome, changed, streamErr := applyStreamGuardrails(
				ctx,
				definition,
				req,
				emitter,
				types.MessageDelta{
					RunID:   event.Delta.RunID,
					Role:    event.Delta.Role,
					Content: event.Delta.Content,
				},
				chunkIndex,
				bufferedContent,
			)
			decisions = append(decisions, streamDecisions...)
			if streamErr != nil {
				return agent.ModelResponse{}, decisions, true, streamErr
			}

			switch outcome {
			case streamChunkEmit:
				bufferedContent += delta.Content
				if delta.Role == "" {
					delta.Role = types.RoleAssistant
				}
				if delta.RunID == "" {
					delta.RunID = req.RunID
				}
				emitter.emit(ctx, agent.EventAgentDelta, false, func(out *agent.Event) {
					cloned := delta
					out.Delta = &cloned
				})
			case streamChunkDrop:
			}

			if changed {
				sawStreamAdjustment = true
			}
		}

		if event.ToolCall != nil {
			response.ToolCalls = append(response.ToolCalls, agent.ModelToolCall{
				ID:    event.ToolCall.ID,
				Name:  event.ToolCall.Name,
				Input: cloneMap(event.ToolCall.Input),
			})
		}

		if event.Message != nil {
			response.Message = types.CloneMessage(*event.Message)
			if response.Message.Role == "" {
				response.Message.Role = types.RoleAssistant
			}
		}

		if event.Done {
			break
		}
	}

	if !sawEvent {
		return agent.ModelResponse{}, nil, false, nil
	}

	if len(response.ToolCalls) > 0 {
		return response, decisions, true, nil
	}

	if sawDelta && (sawStreamAdjustment || response.Message.Content == "") {
		response.Message = types.Message{
			Role:    types.RoleAssistant,
			Content: bufferedContent,
		}
	}

	if response.Message.Role == "" && (sawDelta || len(response.Message.ToolCalls) == 0) {
		response.Message.Role = types.RoleAssistant
	}

	return response, decisions, true, nil
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
	event.Metadata = e.enrichMetadata(event)

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
				if recovered := recover(); recovered != nil {
					logger.FromContext(ctx).ErrorContext(ctx, "agent.hook_panic",
						"component", "agent",
						"event_type", string(event.Type),
						"agent_id", e.definition.Descriptor.ID,
						"agent_name", e.definition.Descriptor.Name,
						"run_id", e.req.RunID,
						"panic", fmt.Sprint(recovered),
					)
				}
			}()
			h.OnEvent(ctx, cloneAgentEvent(event))
		}(hook)
	}
}

func (e *eventEmitter) enrichMetadata(event agent.Event) types.Metadata {
	metadata := types.MergeMetadata(event.Metadata, types.Metadata{
		"component":  "agent",
		"event_type": string(event.Type),
		"agent_id":   e.definition.Descriptor.ID,
		"agent_name": e.definition.Descriptor.Name,
		"run_id":     e.req.RunID,
		"session_id": e.req.SessionID,
	})

	if event.ToolCall != nil {
		metadata = types.MergeMetadata(metadata, types.Metadata{
			"tool_name":    event.ToolCall.Call.Name,
			"tool_call_id": event.ToolCall.Call.ID,
			"tool_status":  string(event.ToolCall.Status),
		})
	}
	if event.Guardrail != nil {
		metadata = types.MergeMetadata(metadata, types.Metadata{
			"phase":          string(event.Guardrail.Decision.Phase),
			"guardrail_name": event.Guardrail.Decision.Name,
			"action":         string(event.Guardrail.Decision.Action),
		})
		metadata = types.MergeMetadata(metadata, event.Guardrail.Decision.Metadata)
	}
	if event.Response != nil {
		metadata = types.MergeMetadata(metadata, types.Metadata{
			"response_run_id": event.Response.RunID,
		})
	}
	return metadata
}

func cloneAgentEvent(event agent.Event) agent.Event {
	cloned := agent.Event{
		Sequence:  event.Sequence,
		Type:      event.Type,
		RunID:     event.RunID,
		AgentID:   event.AgentID,
		SessionID: event.SessionID,
		Time:      event.Time,
		Err:       event.Err,
		Metadata:  types.CloneMetadata(event.Metadata),
	}
	if event.Delta != nil {
		delta := *event.Delta
		cloned.Delta = &delta
	}
	if event.ToolCall != nil {
		toolEvent := *event.ToolCall
		toolEvent.Call = agent.ToolCallRecord{
			ID:       event.ToolCall.Call.ID,
			Name:     event.ToolCall.Call.Name,
			Input:    cloneMap(event.ToolCall.Call.Input),
			Output:   cloneToolResult(event.ToolCall.Call.Output),
			Duration: event.ToolCall.Call.Duration,
		}
		cloned.ToolCall = &toolEvent
	}
	if event.Guardrail != nil {
		guardrailEvent := *event.Guardrail
		guardrailEvent.Decision.Metadata = types.CloneMetadata(event.Guardrail.Decision.Metadata)
		cloned.Guardrail = &guardrailEvent
	}
	if event.Response != nil {
		response := *event.Response
		response.Message = types.CloneMessage(event.Response.Message)
		response.ToolCalls = cloneToolCallRecords(event.Response.ToolCalls)
		response.GuardrailDecisions = cloneGuardrailDecisions(event.Response.GuardrailDecisions)
		response.Metadata = types.CloneMetadata(event.Response.Metadata)
		cloned.Response = &response
	}
	return cloned
}

func preserveCorrelationMetadata(metadata, reqMetadata types.Metadata) types.Metadata {
	for _, key := range []string{"trace_id", "span_id", "parent_span_id"} {
		if value := reqMetadata[key]; value != "" {
			if metadata == nil {
				metadata = types.Metadata{}
			}
			metadata[key] = value
		}
	}
	return metadata
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
			Context: guardrail.Context{
				Phase:     guardrail.PhaseInput,
				AgentID:   definition.Descriptor.ID,
				AgentName: definition.Descriptor.Name,
				RunID:     req.RunID,
				SessionID: req.SessionID,
				Metadata:  types.CloneMetadata(req.Metadata),
			},
			Messages: types.CloneMessages(messages),
		})
		if err != nil {
			return nil, nil, classifyError("guardrail.input", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}
		if err := validateGuardrailDecision(guardrail.PhaseInput, decision); err != nil {
			return nil, decisions, classifyError("guardrail.input", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}

		normalized := normalizeGuardrailDecision(agent.GuardrailPhaseInput, decision, rule)
		decisions = append(decisions, normalized)
		emitter.emit(ctx, agent.EventGuardrail, false, func(event *agent.Event) {
			event.Guardrail = &agent.GuardrailEvent{Decision: normalized}
		})

		switch decision.Action {
		case guardrail.ActionAllow:
		case guardrail.ActionTransform:
			messages = types.CloneMessages(decision.Messages)
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
			Context: guardrail.Context{
				Phase:     guardrail.PhaseOutput,
				AgentID:   definition.Descriptor.ID,
				AgentName: definition.Descriptor.Name,
				RunID:     req.RunID,
				SessionID: req.SessionID,
				Metadata:  types.CloneMetadata(req.Metadata),
			},
			Message: types.CloneMessage(out),
		})
		if err != nil {
			return types.Message{}, nil, classifyError("guardrail.output", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}
		if err := validateGuardrailDecision(guardrail.PhaseOutput, decision); err != nil {
			return types.Message{}, decisions, classifyError("guardrail.output", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}

		normalized := normalizeGuardrailDecision(agent.GuardrailPhaseOutput, decision, rule)
		decisions = append(decisions, normalized)
		emitter.emit(ctx, agent.EventGuardrail, false, func(event *agent.Event) {
			event.Guardrail = &agent.GuardrailEvent{Decision: normalized}
		})

		switch decision.Action {
		case guardrail.ActionAllow:
		case guardrail.ActionTransform:
			out = types.CloneMessage(*decision.Message)
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

type streamChunkOutcome uint8

const (
	streamChunkEmit streamChunkOutcome = iota
	streamChunkDrop
)

func applyStreamGuardrails(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	delta types.MessageDelta,
	chunkIndex int64,
	bufferedContent string,
) (types.MessageDelta, []agent.GuardrailDecision, streamChunkOutcome, bool, error) {
	current := cloneMessageDelta(delta)
	if current.Role == "" {
		current.Role = types.RoleAssistant
	}
	if current.RunID == "" {
		current.RunID = req.RunID
	}

	decisions := make([]agent.GuardrailDecision, 0, len(definition.StreamGuardrails))
	for _, rule := range definition.StreamGuardrails {
		if rule == nil {
			continue
		}

		decision, err := rule.CheckStream(ctx, guardrail.StreamRequest{
			Context: guardrail.Context{
				Phase:     guardrail.PhaseStream,
				AgentID:   definition.Descriptor.ID,
				AgentName: definition.Descriptor.Name,
				RunID:     req.RunID,
				SessionID: req.SessionID,
				Metadata:  types.CloneMetadata(req.Metadata),
			},
			ChunkIndex:      chunkIndex,
			Delta:           cloneMessageDelta(current),
			BufferedContent: bufferedContent,
		})
		if err != nil {
			return types.MessageDelta{}, decisions, streamChunkEmit, false, classifyError("guardrail.stream", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}
		if err := validateGuardrailDecision(guardrail.PhaseStream, decision); err != nil {
			return types.MessageDelta{}, decisions, streamChunkEmit, false, classifyError("guardrail.stream", definition.Descriptor.ID, req.RunID, err, agent.ErrorKindInternal)
		}

		normalized := normalizeStreamGuardrailDecision(decision, rule, chunkIndex)
		decisions = append(decisions, normalized)
		emitter.emit(ctx, agent.EventGuardrail, false, func(event *agent.Event) {
			event.Guardrail = &agent.GuardrailEvent{Decision: normalized}
		})

		switch decision.Action {
		case guardrail.ActionAllow:
		case guardrail.ActionTransform:
			current = cloneMessageDelta(*decision.Delta)
			if current.Role == "" {
				current.Role = delta.Role
			}
			if current.Role == "" {
				current.Role = types.RoleAssistant
			}
			if current.RunID == "" {
				current.RunID = delta.RunID
			}
			if current.RunID == "" {
				current.RunID = req.RunID
			}
		case guardrail.ActionDrop:
			return types.MessageDelta{}, decisions, streamChunkDrop, true, nil
		case guardrail.ActionAbort:
			blockErr := &agent.Error{
				Kind:    agent.ErrorKindGuardrailBlocked,
				Op:      "guardrail.stream",
				AgentID: definition.Descriptor.ID,
				RunID:   req.RunID,
				Cause:   fmt.Errorf("%w: %s", agent.ErrGuardrailBlocked, normalized.Reason),
			}
			return types.MessageDelta{}, decisions, streamChunkEmit, true, blockErr
		}
	}

	return current, decisions, streamChunkEmit, current != delta, nil
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

func normalizeStreamGuardrailDecision(
	decision guardrail.Decision,
	rule any,
	chunkIndex int64,
) agent.GuardrailDecision {
	normalized := normalizeGuardrailDecision(agent.GuardrailPhaseStream, decision, rule)
	metadata := types.CloneMetadata(normalized.Metadata)
	if metadata == nil {
		metadata = types.Metadata{}
	}
	metadata["chunk_index"] = fmt.Sprintf("%d", chunkIndex)
	normalized.Metadata = metadata
	return normalized
}

func validateGuardrailDecision(phase guardrail.Phase, decision guardrail.Decision) error {
	switch decision.Action {
	case guardrail.ActionAllow:
		return nil
	case guardrail.ActionTransform:
		switch phase {
		case guardrail.PhaseInput:
			if decision.Messages == nil {
				return errors.New("input guardrail transform requires messages")
			}
		case guardrail.PhaseStream:
			if decision.Delta == nil {
				return errors.New("stream guardrail transform requires delta")
			}
			if decision.Delta.Role != "" && decision.Delta.Role != types.RoleAssistant {
				return errors.New("stream guardrail transform requires assistant delta role")
			}
		case guardrail.PhaseOutput:
			if decision.Message == nil {
				return errors.New("output guardrail transform requires message")
			}
			if decision.Message.Role != "" && decision.Message.Role != types.RoleAssistant {
				return errors.New("output guardrail transform requires assistant message role")
			}
		default:
			return errors.New("unknown guardrail phase")
		}
		return nil
	case guardrail.ActionBlock:
		if phase == guardrail.PhaseStream {
			return errors.New("stream guardrail cannot block")
		}
		return nil
	case guardrail.ActionDrop:
		if phase != guardrail.PhaseStream {
			return errors.New("drop is only valid for stream guardrails")
		}
		return nil
	case guardrail.ActionAbort:
		if phase != guardrail.PhaseStream {
			return errors.New("abort is only valid for stream guardrails")
		}
		return nil
	default:
		return fmt.Errorf("unknown guardrail action %q", decision.Action)
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

func (e *engine) invokePublicToolCall(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	effectiveTools map[string]tool.Tool,
	modelToolCall agent.ModelToolCall,
	step int,
	index int,
	toolCalls *[]agent.ToolCallRecord,
	toolUsed *bool,
) (agent.ToolCallRecord, error) {
	if isReservedReasoningToolName(modelToolCall.Name) {
		toolErr := fmt.Errorf("reserved internal tool %q cannot be invoked publicly", modelToolCall.Name)
		return agent.ToolCallRecord{}, classifyError("tool.resolve", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool)
	}

	selectedTool, ok := effectiveTools[modelToolCall.Name]
	if !ok {
		toolErr := fmt.Errorf("tool %q is not available for this run", modelToolCall.Name)
		return agent.ToolCallRecord{}, classifyError("tool.resolve", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool)
	}

	toolDescriptor := tool.DescriptorOf(selectedTool)
	callID := modelToolCall.ID
	if callID == "" {
		callID = newToolCallID(req.RunID, step, index)
	}

	*toolUsed = true
	callRecord := agent.ToolCallRecord{
		ID:    callID,
		Name:  toolDescriptor.Name,
		Input: cloneMap(modelToolCall.Input),
	}

	emitter.emit(ctx, agent.EventToolCall, false, func(event *agent.Event) {
		event.ToolCall = &agent.ToolCallEvent{
			Call:   cloneToolCallRecord(callRecord),
			Status: agent.ToolCallStarted,
		}
	})

	startedAt := time.Now()
	result, toolErr := tool.Invoke(ctx, selectedTool, tool.Call{
		ID:        callID,
		ToolName:  toolDescriptor.Name,
		RunID:     req.RunID,
		SessionID: req.SessionID,
		AgentID:   definition.Descriptor.ID,
		Input:     cloneMap(modelToolCall.Input),
		Metadata:  types.CloneMetadata(req.Metadata),
	})
	callRecord.Duration = time.Since(startedAt)

	if toolErr != nil {
		classified := classifyError("tool.call", definition.Descriptor.ID, req.RunID, toolErr, agent.ErrorKindTool)
		emitter.emit(ctx, agent.EventToolResult, false, func(event *agent.Event) {
			event.ToolCall = &agent.ToolCallEvent{
				Call:   cloneToolCallRecord(callRecord),
				Status: agent.ToolCallFailed,
			}
			event.Err = classified
		})
		return agent.ToolCallRecord{}, classified
	}

	callRecord.Output = cloneToolResult(result)
	*toolCalls = append(*toolCalls, callRecord)

	emitter.emit(ctx, agent.EventToolResult, false, func(event *agent.Event) {
		event.ToolCall = &agent.ToolCallEvent{
			Call:   cloneToolCallRecord(callRecord),
			Status: agent.ToolCallSucceeded,
		}
	})

	return callRecord, nil
}

func (e *engine) executeSuggestedToolCall(
	ctx context.Context,
	definition agent.Definition,
	req agent.Request,
	emitter *eventEmitter,
	effectiveTools map[string]tool.Tool,
	conversation *[]types.Message,
	working memory.WorkingSet,
	toolCalls *[]agent.ToolCallRecord,
	toolUsed *bool,
	step int,
	action reasoningAction,
) error {
	suggested := agent.ModelToolCall{
		ID:    newToolCallID(req.RunID, step, len(*toolCalls)),
		Name:  action.toolName,
		Input: cloneMap(action.toolInput),
	}

	appendAssistantToolCallMessage(conversation, working, []agent.ModelToolCall{suggested})

	callRecord, err := e.invokePublicToolCall(
		ctx,
		definition,
		req,
		emitter,
		effectiveTools,
		suggested,
		step,
		len(*toolCalls),
		toolCalls,
		toolUsed,
	)
	if err != nil {
		return err
	}

	working.AddRecord(memory.Record{
		Kind: "tool_call",
		Name: callRecord.Name,
		Data: map[string]any{
			"call_id": callRecord.ID,
			"input":   cloneMap(callRecord.Input),
			"output":  cloneToolResult(callRecord.Output),
		},
	})

	toolMessage := types.Message{
		Role:       types.RoleTool,
		Name:       callRecord.Name,
		ToolCallID: callRecord.ID,
		Content:    toolResultContent(callRecord.Output),
		Metadata:   types.CloneMetadata(callRecord.Output.Metadata),
	}
	*conversation = append(*conversation, toolMessage)
	working.AddMessage(toolMessage)
	return nil
}

func appendAssistantToolCallMessage(conversation *[]types.Message, working memory.WorkingSet, calls []agent.ModelToolCall) {
	if len(calls) == 0 {
		return
	}
	message := types.Message{
		Role:      types.RoleAssistant,
		ToolCalls: toTypesToolCalls(cloneModelToolCalls(calls)),
	}
	*conversation = append(*conversation, message)
	working.AddMessage(message)
}

func appendInternalReasoningNote(conversation []types.Message, note string) []types.Message {
	note = strings.TrimSpace(note)
	if note == "" {
		return conversation
	}
	return append(conversation, types.Message{
		Role:    types.RoleSystem,
		Content: note,
	})
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
		descriptor := tool.DescriptorOf(registeredTool)
		if isReservedReasoningToolName(descriptor.Name) {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[descriptor.Name]; !ok {
				continue
			}
		}
		out[descriptor.Name] = registeredTool
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
		descriptor := tool.DescriptorOf(registeredTool)
		out = append(out, agent.ToolSpec{
			Name:        descriptor.Name,
			Description: descriptor.Description,
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
		out[key] = cloneValue(value)
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

func cloneMessageDelta(delta types.MessageDelta) types.MessageDelta {
	return types.MessageDelta{
		RunID:   delta.RunID,
		Role:    delta.Role,
		Content: delta.Content,
	}
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
		Value:    cloneValue(result.Value),
		Content:  result.Content,
		Metadata: types.CloneMetadata(result.Metadata),
	}
}

func toolResultContent(result tool.Result) string {
	if result.Content != "" {
		return result.Content
	}
	if result.Value == nil {
		return ""
	}

	raw, err := json.Marshal(result.Value)
	if err != nil {
		return fmt.Sprintf("%v", result.Value)
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

func cloneSlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}

	out := make([]any, len(in))
	for index, value := range in {
		out[index] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		return cloneSlice(value)
	default:
		return value
	}
}

func newToolCallID(runID string, step, index int) string {
	if runID == "" {
		return fmt.Sprintf("tool-call-%d-%d", step+1, index+1)
	}
	return fmt.Sprintf("%s-tool-%d-%d", runID, step+1, index+1)
}
