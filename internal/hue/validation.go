package hue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var compiledSchemas sync.Map

func CompileRequestSchema(kind, method string) (*jsonschema.Schema, error) {
	key := kind + "/" + method
	if cached, ok := compiledSchemas.Load(key); ok {
		return cached.(*jsonschema.Schema), nil
	}
	capability, err := Capability(kind)
	if err != nil {
		return nil, err
	}
	body := capability.RequestSchemas[method]
	if len(body) == 0 {
		return nil, &Problem{Code: "schema_unavailable", Message: fmt.Sprintf("no request schema for %s %s", method, kind)}
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://huego.local/"+key, document); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("https://huego.local/" + key)
	if err != nil {
		return nil, err
	}
	compiledSchemas.Store(key, schema)
	return schema, nil
}

func ValidateSchema(schemaJSON, payload json.RawMessage) error {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://huego.local/configuration", document); err != nil {
		return err
	}
	schema, err := compiler.Compile("https://huego.local/configuration")
	if err != nil {
		return err
	}
	return validatePayload(schema, payload)
}

func ValidateRequest(kind, method string, body json.RawMessage) error {
	schema, err := CompileRequestSchema(kind, method)
	if err != nil {
		return err
	}
	return validatePayload(schema, body)
}

func validatePayload(schema *jsonschema.Schema, body json.RawMessage) error {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return &Problem{Code: "invalid_payload", Message: err.Error()}
	}
	if err := schema.Validate(value); err != nil {
		var validation *jsonschema.ValidationError
		if errors.As(err, &validation) {
			return &Problem{Code: "validation_failed", Message: err.Error(), Details: validation.BasicOutput()}
		}
		return err
	}
	return nil
}
