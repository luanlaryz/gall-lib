package tool

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type registry struct {
	mu       sync.RWMutex
	tools    map[string]registryEntry
	toolkits map[string]ToolkitDescriptor
}

type registryEntry struct {
	tool       Tool
	descriptor Descriptor
}

func (r *registry) Register(tools ...Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nextTools := cloneRegistryEntries(r.tools)
	for _, candidate := range tools {
		entry, err := newRegistryEntry(candidate, toolkitOrigin{})
		if err != nil {
			return err
		}
		if conflict, exists := nextTools[entry.descriptor.Name]; exists {
			return newNameConflictError(
				"register",
				entry.descriptor.Name,
				entry.descriptor.Toolkit,
				conflict.descriptor.Toolkit,
			)
		}
		nextTools[entry.descriptor.Name] = entry
	}

	r.tools = nextTools
	return nil
}

func (r *registry) RegisterToolkits(toolkits ...Toolkit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nextTools := cloneRegistryEntries(r.tools)
	nextToolkits := cloneToolkitDescriptors(r.toolkits)

	for _, candidate := range toolkits {
		toolkitDescriptor, entries, err := buildToolkitEntries(candidate)
		if err != nil {
			return err
		}
		if _, exists := nextToolkits[toolkitDescriptor.Name]; exists {
			return newNameConflictError("register_toolkits", "", toolkitDescriptor.Name, toolkitDescriptor.Name)
		}
		for _, entry := range entries {
			if conflict, exists := nextTools[entry.descriptor.Name]; exists {
				return newNameConflictError(
					"register_toolkits",
					entry.descriptor.Name,
					toolkitDescriptor.Name,
					conflict.descriptor.Toolkit,
				)
			}
		}

		nextToolkits[toolkitDescriptor.Name] = toolkitDescriptor
		for _, entry := range entries {
			nextTools[entry.descriptor.Name] = entry
		}
	}

	r.tools = nextTools
	r.toolkits = nextToolkits
	return nil
}

func (r *registry) Resolve(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.tools[name]
	if !ok {
		return nil, newError(ErrorKindNotFound, "resolve", name, "", "", fmt.Errorf("%w: tool %q is not registered", ErrToolNotFound, name))
	}
	return entry.tool, nil
}

func (r *registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Descriptor, 0, len(r.tools))
	for _, entry := range r.tools {
		out = append(out, cloneDescriptor(entry.descriptor))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (r *registry) ListToolkits() []ToolkitDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ToolkitDescriptor, 0, len(r.toolkits))
	for _, descriptor := range r.toolkits {
		out = append(out, cloneToolkitDescriptor(descriptor))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

type toolkitOrigin struct {
	name      string
	namespace string
}

func buildToolkitEntries(toolkit Toolkit) (ToolkitDescriptor, []registryEntry, error) {
	if toolkit == nil {
		return ToolkitDescriptor{}, nil, newError(
			ErrorKindInvalidToolkit,
			"register_toolkits",
			"",
			"",
			"",
			fmt.Errorf("%w: toolkit cannot be nil", ErrInvalidToolkit),
		)
	}

	name := toolkit.Name()
	description := toolkit.Description()
	namespace := toolkit.Namespace()
	if err := validateToolkitName(name); err != nil {
		return ToolkitDescriptor{}, nil, newError(ErrorKindInvalidToolkit, "register_toolkits", "", name, "", err)
	}
	if description == "" {
		return ToolkitDescriptor{}, nil, newError(
			ErrorKindInvalidToolkit,
			"register_toolkits",
			"",
			name,
			"",
			fmt.Errorf("%w: toolkit description is required", ErrInvalidToolkit),
		)
	}
	if namespace != "" {
		if err := validateName(namespace, "namespace"); err != nil {
			return ToolkitDescriptor{}, nil, newError(ErrorKindInvalidToolkit, "register_toolkits", "", name, "", err)
		}
	}

	tools := toolkit.Tools()
	localNames := make(map[string]struct{}, len(tools))
	entries := make([]registryEntry, 0, len(tools))
	for _, candidate := range tools {
		if candidate == nil {
			return ToolkitDescriptor{}, nil, newError(
				ErrorKindInvalidToolkit,
				"register_toolkits",
				"",
				name,
				"",
				fmt.Errorf("%w: toolkit contains nil tool", ErrInvalidToolkit),
			)
		}

		localName := candidate.Name()
		if _, exists := localNames[localName]; exists {
			return ToolkitDescriptor{}, nil, newError(
				ErrorKindInvalidToolkit,
				"register_toolkits",
				localName,
				name,
				"",
				fmt.Errorf("%w: duplicate toolkit tool %q", ErrInvalidToolkit, localName),
			)
		}
		localNames[localName] = struct{}{}

		entry, err := newRegistryEntry(candidate, toolkitOrigin{name: name, namespace: namespace})
		if err != nil {
			return ToolkitDescriptor{}, nil, newError(ErrorKindInvalidToolkit, "register_toolkits", localName, name, "", err)
		}
		entries = append(entries, entry)
	}

	return ToolkitDescriptor{
		Name:        name,
		Description: description,
		Namespace:   namespace,
		ToolCount:   len(entries),
	}, entries, nil
}

func newRegistryEntry(candidate Tool, origin toolkitOrigin) (registryEntry, error) {
	descriptor, err := validateToolDefinition(candidate, origin)
	if err != nil {
		return registryEntry{}, err
	}

	frozen := resolvedTool{
		descriptor: descriptor,
		delegate:   candidate,
	}
	return registryEntry{
		tool:       frozen,
		descriptor: descriptor,
	}, nil
}

type resolvedTool struct {
	descriptor Descriptor
	delegate   Tool
}

func (t resolvedTool) Name() string {
	return t.descriptor.LocalName
}

func (t resolvedTool) Description() string {
	return t.descriptor.Description
}

func (t resolvedTool) InputSchema() Schema {
	return cloneSchema(t.descriptor.InputSchema)
}

func (t resolvedTool) OutputSchema() Schema {
	return cloneSchema(t.descriptor.OutputSchema)
}

func (t resolvedTool) Call(ctx context.Context, call Call) (Result, error) {
	return t.delegate.Call(ctx, call)
}

func (t resolvedTool) Descriptor() Descriptor {
	return cloneDescriptor(t.descriptor)
}

func cloneRegistryEntries(in map[string]registryEntry) map[string]registryEntry {
	if len(in) == 0 {
		return make(map[string]registryEntry)
	}

	out := make(map[string]registryEntry, len(in))
	for key, value := range in {
		out[key] = registryEntry{
			tool:       value.tool,
			descriptor: cloneDescriptor(value.descriptor),
		}
	}
	return out
}

func cloneToolkitDescriptors(in map[string]ToolkitDescriptor) map[string]ToolkitDescriptor {
	if len(in) == 0 {
		return make(map[string]ToolkitDescriptor)
	}

	out := make(map[string]ToolkitDescriptor, len(in))
	for key, value := range in {
		out[key] = cloneToolkitDescriptor(value)
	}
	return out
}

func newNameConflictError(op, toolName, toolkitName, existingToolkit string) error {
	cause := fmt.Errorf("%w: effective name conflict", ErrNameConflict)
	switch {
	case toolName != "" && toolkitName != "" && existingToolkit != "":
		cause = fmt.Errorf("%w: tool %q from toolkit %q conflicts with toolkit %q", ErrNameConflict, toolName, toolkitName, existingToolkit)
	case toolName != "" && toolkitName != "":
		cause = fmt.Errorf("%w: tool %q conflicts while registering toolkit %q", ErrNameConflict, toolName, toolkitName)
	case toolName != "":
		cause = fmt.Errorf("%w: tool %q is already registered", ErrNameConflict, toolName)
	case toolkitName != "":
		cause = fmt.Errorf("%w: toolkit %q is already registered", ErrNameConflict, toolkitName)
	}

	return newError(ErrorKindNameConflict, op, toolName, toolkitName, "", cause)
}
