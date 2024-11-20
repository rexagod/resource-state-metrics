/*
Copyright 2024 The Kubernetes resource-state-metrics Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resolver

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
	"k8s.io/klog/v2"
)

// CELResolver represents a resolver for CEL expressions.
type CELResolver struct {
	logger klog.Logger
}

// CELResolver implements the Resolver interface.
var _ Resolver = &CELResolver{}

// NewCELResolver returns a new CEL resolver.
func NewCELResolver(logger klog.Logger) *CELResolver {
	return &CELResolver{logger: logger}
}

// costEstimator helps estimate the runtime cost of CEL queries.
type costEstimator struct{}

// costEstimator implements the ActualCostEstimator interface.
var _ interpreter.ActualCostEstimator = costEstimator{}

// CallCost helps set the runtime cost for CEL queries on a per-function basis. This affects `ActualCost()` directly.
// function: The function name.
// args: The arguments passed to the function.
// result: The return value of the function.
//
//nolint:revive // Keep the unused args for aforementioned reference.
func (ce costEstimator) CallCost(function, _ string, args []ref.Val, result ref.Val) *uint64 {
	estimatedCost := uint64(1)
	customFunctionsCosts := map[string]uint64{}
	estimatedCost += customFunctionsCosts[function]

	return &estimatedCost
}

// Resolve resolves the given query against the given unstructured object.
func (cr *CELResolver) Resolve(query string, unstructuredObjectMap map[string]interface{}) map[string]string {
	logger := cr.logger.WithValues("query", query)

	// Create a custom CEL environment.
	env, err := cel.NewEnv(
		cel.CrossTypeNumericComparisons(true),
		cel.DefaultUTCTimeZone(true),
		cel.EagerlyValidateDeclarations(true),
	)
	if err != nil {
		logger.Error(fmt.Errorf("error creating CEL environment: %w", err), "ignoring resolution for query")

		return map[string]string{query: query}
	}

	// Parse.
	ast, iss := env.Parse(query)
	if iss.Err() != nil {
		logger.Error(fmt.Errorf("error parsing CEL query: %w", iss.Err()), "ignoring resolution for query")

		return map[string]string{query: query}
	}

	// Compile.
	// costLimit gives ~0.1s for each CEL expression validation call.
	const costLimit = 1000000
	var program cel.Program
	program, err = env.Program(
		ast,
		cel.CostLimit(costLimit),
		cel.CostTracking(new(costEstimator)),
	)
	if err != nil {
		logger.Error(fmt.Errorf("error compiling CEL query: %w", err), "ignoring resolution for query")

		return map[string]string{query: query}
	}

	// Inject the object and evaluate.
	var out ref.Val
	var evalDetails *cel.EvalDetails
	out, evalDetails, err = program.Eval(map[string]interface{}{
		"o" /* Queries will follow the format: o.<A>.<AB>.<ABC>... */ : unstructuredObjectMap,
	})
	logger = logger.WithValues(
		"costLimit", costLimit,
	)
	if evalDetails != nil {
		logger = logger.WithValues(
			"queryCost", *evalDetails.ActualCost(),
		)
	}
	if err != nil {
		logger.V(1).Info("ignoring resolution for query", "info", err)

		return map[string]string{query: query}
	}
	logger.V(4).Info("CEL query runtime cost")

	m := map[string]string{}
	switch out.Type() {
	case types.BoolType, types.DoubleType, types.IntType, types.StringType, types.UintType:

		// If the output is a primitive type, return the query and the resolved value.
		m = map[string]string{query: fmt.Sprintf("%v", out.Value())}
	case types.MapType:
		m = cr.resolveMap(&out)
	case types.ListType:
		m = cr.resolveList(&out)
	default:
		logger.Error(fmt.Errorf("unsupported output type %q", out.Type()), "ignoring resolution for query")
	}

	if m == nil {
		m = map[string]string{query: query}
	}

	return m
}

func (cr *CELResolver) resolveList(out *ref.Val) map[string]string {
	m := map[string]string{}
	outList, ok := (*out).Value().([]interface{})
	if !ok {
		cr.logger.V(1).Error(errors.New("error casting output to []interface{}"), "ignoring resolution for query")

		return nil
	}
	for i, v := range outList {
		switch v.(type) {
		case string, int, uint, float64, bool:
			m[strconv.Itoa(i)] = fmt.Sprintf("%v", v)
		default:
			cr.logger.V(1).Error(fmt.Errorf("encountered composite value %q at index %d, skipping", v, i), "ignoring resolution for query")
		}
	}

	return m
}

func (cr *CELResolver) resolveMap(out *ref.Val) map[string]string {
	m := map[string]string{}
	outMap, ok := (*out).Value().(map[string]interface{})
	if !ok {
		cr.logger.V(1).Error(errors.New("error casting output to map[string]interface{}"), "ignoring resolution for query")

		return nil
	}
	for k, v := range outMap {
		switch v.(type) {
		case string, int, uint, float64, bool:

			// Even in cases where the parent and immediate child have the same key, the "o" prefix in CEL queries will prevent any collision.
			m[k] = fmt.Sprintf("%v", v)
		default:
			cr.logger.V(1).Error(fmt.Errorf("encountered composite value %q at key %q, skipping", v, k), "ignoring resolution for query")
		}
	}

	return m
}
