package tool

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

var supportedSchemaTypes = map[string]struct{}{
	"array":   {},
	"boolean": {},
	"integer": {},
	"null":    {},
	"number":  {},
	"object":  {},
	"string":  {},
}

func validateToolDefinition(candidate Tool, origin toolkitOrigin) (Descriptor, error) {
	if candidate == nil {
		return Descriptor{}, newError(
			ErrorKindInvalidTool,
			"register",
			"",
			origin.name,
			"",
			fmt.Errorf("%w: tool cannot be nil", ErrInvalidTool),
		)
	}

	descriptor := DescriptorOf(candidate)
	descriptor.LocalName = candidate.Name()
	descriptor.Description = strings.TrimSpace(candidate.Description())
	descriptor.InputSchema = cloneSchema(candidate.InputSchema())
	descriptor.OutputSchema = cloneSchema(candidate.OutputSchema())

	if origin.name != "" {
		descriptor.Toolkit = origin.name
		descriptor.Namespace = origin.namespace
		descriptor.Name = effectiveName(origin.namespace, descriptor.LocalName)
	} else {
		if _, ok := candidate.(describedTool); !ok || descriptor.Name == "" {
			descriptor.Name = effectiveName(descriptor.Namespace, descriptor.LocalName)
		}
	}

	if err := validateName(descriptor.LocalName, "tool name"); err != nil {
		return Descriptor{}, newError(ErrorKindInvalidTool, "register", descriptor.LocalName, descriptor.Toolkit, "", err)
	}
	if descriptor.Description == "" {
		return Descriptor{}, newError(
			ErrorKindInvalidTool,
			"register",
			descriptor.Name,
			descriptor.Toolkit,
			"",
			fmt.Errorf("%w: tool description is required", ErrInvalidTool),
		)
	}
	if descriptor.Toolkit != "" {
		if err := validateToolkitName(descriptor.Toolkit); err != nil {
			return Descriptor{}, newError(ErrorKindInvalidTool, "register", descriptor.Name, descriptor.Toolkit, "", err)
		}
	}
	if descriptor.Namespace != "" {
		if err := validateName(descriptor.Namespace, "namespace"); err != nil {
			return Descriptor{}, newError(ErrorKindInvalidTool, "register", descriptor.Name, descriptor.Toolkit, "", err)
		}
	}
	if expected := effectiveName(descriptor.Namespace, descriptor.LocalName); descriptor.Name != expected {
		return Descriptor{}, newError(
			ErrorKindInvalidTool,
			"register",
			descriptor.Name,
			descriptor.Toolkit,
			"",
			fmt.Errorf("%w: effective name %q does not match namespace/local name", ErrInvalidTool, descriptor.Name),
		)
	}
	if err := validateInputSchema(descriptor.InputSchema); err != nil {
		return Descriptor{}, newError(ErrorKindInvalidSchema, "register", descriptor.Name, descriptor.Toolkit, "", err)
	}
	if err := validateOutputSchema(descriptor.OutputSchema); err != nil {
		return Descriptor{}, newError(ErrorKindInvalidSchema, "register", descriptor.Name, descriptor.Toolkit, "", err)
	}

	return descriptor, nil
}

func validateToolkitName(name string) error {
	return validateName(name, "toolkit name")
}

func validateName(value, label string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidTool, label)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%w: %s %q must not contain surrounding whitespace", ErrInvalidTool, label, value)
	}
	if !namePattern.MatchString(value) {
		return fmt.Errorf("%w: %s %q must match %s", ErrInvalidTool, label, value, namePattern.String())
	}
	return nil
}

func validateInputSchema(schema Schema) error {
	if schema.Type != "object" {
		return fmt.Errorf("%w: input schema type must be object", ErrInvalidSchema)
	}
	return validateSchema(schema, "input")
}

func validateOutputSchema(schema Schema) error {
	if strings.TrimSpace(schema.Type) == "" {
		return fmt.Errorf("%w: output schema type is required", ErrInvalidSchema)
	}
	return validateSchema(schema, "output")
}

func validateSchema(schema Schema, path string) error {
	if _, ok := supportedSchemaTypes[schema.Type]; !ok {
		return fmt.Errorf("%w: %s schema type %q is not supported", ErrInvalidSchema, path, schema.Type)
	}

	switch schema.Type {
	case "object":
		seenRequired := make(map[string]struct{}, len(schema.Required))
		for _, name := range schema.Required {
			if _, exists := seenRequired[name]; exists {
				return fmt.Errorf("%w: %s schema required field %q is duplicated", ErrInvalidSchema, path, name)
			}
			seenRequired[name] = struct{}{}
			if _, exists := schema.Properties[name]; !exists {
				return fmt.Errorf("%w: %s schema required field %q is not declared", ErrInvalidSchema, path, name)
			}
		}
		for name, property := range schema.Properties {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("%w: %s schema property names cannot be empty", ErrInvalidSchema, path)
			}
			if err := validateSchema(property, path+"."+name); err != nil {
				return err
			}
		}
	case "array":
		if schema.Items != nil {
			if err := validateSchema(*schema.Items, path+"[]"); err != nil {
				return err
			}
		}
	}

	if len(schema.Enum) > 0 {
		if schema.Type != "string" {
			return fmt.Errorf("%w: %s schema enum is only supported for string type", ErrInvalidSchema, path)
		}
		if slices.Contains(schema.Enum, "") {
			return fmt.Errorf("%w: %s schema enum values cannot be empty", ErrInvalidSchema, path)
		}
	}

	return nil
}

func validateValueAgainstSchema(value any, schema Schema, path string) error {
	switch schema.Type {
	case "object":
		objectValue, ok := value.(map[string]any)
		if !ok {
			if value == nil {
				objectValue = nil
			} else {
				return fmt.Errorf("%s must be object, got %T", path, value)
			}
		}

		for _, field := range schema.Required {
			if _, exists := objectValue[field]; !exists {
				return fmt.Errorf("%s.%s is required", path, field)
			}
		}

		allowAdditional := schema.AdditionalProperties != nil && *schema.AdditionalProperties
		for key, item := range objectValue {
			propertySchema, declared := schema.Properties[key]
			if !declared {
				if !allowAdditional {
					return fmt.Errorf("%s.%s is not allowed", path, key)
				}
				if err := validateJSONLike(item, path+"."+key); err != nil {
					return err
				}
				continue
			}
			if err := validateValueAgainstSchema(item, propertySchema, path+"."+key); err != nil {
				return err
			}
		}
		return nil
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be array, got %T", path, value)
		}
		for index, item := range items {
			itemPath := fmt.Sprintf("%s[%d]", path, index)
			if schema.Items != nil {
				if err := validateValueAgainstSchema(item, *schema.Items, itemPath); err != nil {
					return err
				}
				continue
			}
			if err := validateJSONLike(item, itemPath); err != nil {
				return err
			}
		}
		return nil
	case "string":
		stringValue, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be string, got %T", path, value)
		}
		if len(schema.Enum) > 0 && !slices.Contains(schema.Enum, stringValue) {
			return fmt.Errorf("%s must be one of %v", path, schema.Enum)
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be boolean, got %T", path, value)
		}
		return nil
	case "integer":
		if !isInteger(value) {
			return fmt.Errorf("%s must be integer, got %T", path, value)
		}
		return nil
	case "number":
		if !isNumber(value) {
			return fmt.Errorf("%s must be number, got %T", path, value)
		}
		return nil
	case "null":
		if value != nil {
			return fmt.Errorf("%s must be null, got %T", path, value)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported schema type %q", ErrInvalidSchema, schema.Type)
	}
}

func validateJSONLike(value any, path string) error {
	switch value := value.(type) {
	case nil, bool, string:
		return nil
	case int, int8, int16, int32, int64:
		return nil
	case uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("%s must be finite number", path)
		}
		return nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be finite number", path)
		}
		return nil
	case []any:
		for index, item := range value {
			if err := validateJSONLike(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		for key, item := range value {
			if err := validateJSONLike(item, path+"."+key); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s contains non JSON-like value of type %T", path, value)
	}
}

func effectiveName(namespace, localName string) string {
	if namespace == "" {
		return localName
	}
	return namespace + "." + localName
}

func isInteger(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func isNumber(value any) bool {
	if isInteger(value) {
		return true
	}

	switch number := value.(type) {
	case float32:
		return !math.IsNaN(float64(number)) && !math.IsInf(float64(number), 0)
	case float64:
		return !math.IsNaN(number) && !math.IsInf(number, 0)
	default:
		return false
	}
}

func cloneDescriptor(in Descriptor) Descriptor {
	return Descriptor{
		Name:         in.Name,
		LocalName:    in.LocalName,
		Description:  in.Description,
		Toolkit:      in.Toolkit,
		Namespace:    in.Namespace,
		InputSchema:  cloneSchema(in.InputSchema),
		OutputSchema: cloneSchema(in.OutputSchema),
	}
}

func cloneToolkitDescriptor(in ToolkitDescriptor) ToolkitDescriptor {
	return ToolkitDescriptor{
		Name:        in.Name,
		Description: in.Description,
		Namespace:   in.Namespace,
		ToolCount:   in.ToolCount,
	}
}

func cloneSchema(in Schema) Schema {
	out := Schema{
		Type:        in.Type,
		Description: in.Description,
		Required:    append([]string(nil), in.Required...),
		Enum:        append([]string(nil), in.Enum...),
	}
	if len(in.Properties) > 0 {
		out.Properties = make(map[string]Schema, len(in.Properties))
		for key, value := range in.Properties {
			out.Properties[key] = cloneSchema(value)
		}
	}
	if in.Items != nil {
		items := cloneSchema(*in.Items)
		out.Items = &items
	}
	if in.AdditionalProperties != nil {
		value := *in.AdditionalProperties
		out.AdditionalProperties = &value
	}
	return out
}

func isZeroSchema(schema Schema) bool {
	return schema.Type == "" &&
		schema.Description == "" &&
		len(schema.Properties) == 0 &&
		schema.Items == nil &&
		len(schema.Required) == 0 &&
		len(schema.Enum) == 0 &&
		schema.AdditionalProperties == nil
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
