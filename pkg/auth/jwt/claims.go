package jwt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
	"github.com/coreos/go-oidc/v3/oidc"
)

// expressionCostLimit limits the evaluation cost of an expression, e.g. when iterating over large list claims.
const expressionCostLimit = 1_000_000

// decodeClaims returns the claims of a token. Integers are kept exact instead
// of being converted to float64, so large IDs don't lose precision.
func decodeClaims(token *oidc.IDToken) (map[string]any, error) {
	var raw json.RawMessage
	if err := token.Claims(&raw); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var claims map[string]any
	if err := dec.Decode(&claims); err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	if _, err := convertNumbers(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// convertNumbers converts the JSON numbers of a decoded value in place.
func convertNumbers(value any) (any, error) {
	var err error

	switch v := value.(type) {
	case map[string]any:
		for k, e := range v {
			if v[k], err = convertNumbers(e); err != nil {
				return nil, err
			}
		}
	case []any:
		for i, e := range v {
			if v[i], err = convertNumbers(e); err != nil {
				return nil, err
			}
		}
	case json.Number:
		return convertNumber(v)
	}
	return value, nil
}

// convertNumber converts a JSON number to int64 or uint64 if it is an integer,
// otherwise to float64. Integers beyond these types and floats beyond float64
// are rejected, as rounding them could map different values to the same project.
func convertNumber(number json.Number) (any, error) {
	if i, err := number.Int64(); err == nil {
		return i, nil
	}
	if u, err := strconv.ParseUint(number.String(), 10, 64); err == nil {
		return u, nil
	}

	f, err := number.Float64()
	if err != nil || !strings.ContainsAny(number.String(), ".eE") {
		return nil, fmt.Errorf("number %s is out of range", number)
	}
	return f, nil
}

// compileExpression compiles a CEL expression over the "claims" variable and
// checks that it returns one of the given types (or a dynamic value).
func compileExpression(expression string, types ...*cel.Type) (cel.Program, error) {
	env, err := cel.NewEnv(
		cel.Variable("claims", cel.MapType(cel.StringType, cel.DynType)),
		ext.Strings(),
	)
	if err != nil {
		return nil, err
	}

	ast, issues := env.Compile(expression)
	if issues.Err() != nil {
		return nil, issues.Err()
	}

	output := ast.OutputType()
	if !output.IsExactType(cel.DynType) && !slices.ContainsFunc(types, output.IsExactType) {
		return nil, fmt.Errorf("expression returns %s, expected %v", output, types)
	}

	return env.Program(ast, cel.CostLimit(expressionCostLimit))
}

// evalBool evaluates an expression returning a bool.
func evalBool(prg cel.Program, claims map[string]any) (bool, error) {
	out, _, err := prg.Eval(map[string]any{"claims": claims})
	if err != nil {
		return false, err
	}

	ok, isBool := out.Value().(bool)
	if !isBool {
		return false, fmt.Errorf("expression returned %s, expected bool", out.Type())
	}
	return ok, nil
}

// evalStrings evaluates an expression returning a string or a list of strings.
func evalStrings(prg cel.Program, claims map[string]any) ([]string, error) {
	out, _, err := prg.Eval(map[string]any{"claims": claims})
	if err != nil {
		return nil, err
	}

	if value, ok := out.Value().(string); ok {
		return []string{value}, nil
	}

	values, err := out.ConvertToNative(reflect.TypeFor[[]string]())
	if err != nil {
		return nil, fmt.Errorf("expression returned %s, expected string or list of strings", out.Type())
	}
	return values.([]string), nil
}
