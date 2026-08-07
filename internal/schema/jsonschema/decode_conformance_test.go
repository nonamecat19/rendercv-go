//go:build conformance

package jsonschema_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
)

// decodeObject parses upstream's JSON into the port's ordered object, keeping
// the key order the file has.
//
// `encoding/json` into a map would lose that order, which is exactly what the
// differential is checking, so the token stream is walked by hand.
func decodeObject(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()

	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	return decodeFromToken(decoder, token)
}

func decodeFromToken(decoder *json.Decoder, token json.Token) (any, error) {
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			object := jsonschema.NewObject()
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("object key is %T", keyToken)
				}
				value, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				object.Set(key, value)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return object, nil
		case '[':
			values := []any{}
			for decoder.More() {
				value, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return values, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", typed)
	case json.Number:
		integer, err := typed.Int64()
		if err != nil {
			return nil, fmt.Errorf("non-integer number %s", typed)
		}
		return int(integer), nil
	case string, bool, nil:
		return typed, nil
	}
	return nil, fmt.Errorf("unexpected token %T", token)
}
