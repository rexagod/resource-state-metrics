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
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter"
	"k8s.io/klog/v2"
)

// CELResolver represents a resolver for CEL expressions.
type CELResolver struct {
	logger              klog.Logger
	mutex               sync.Mutex
	resolvedFieldParent string
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

// CallCost sets the runtime cost for CEL queries on a per-function basis.
func (ce costEstimator) CallCost(function string, _ string, _ []ref.Val, _ ref.Val) *uint64 {
	customFunctionsCosts := map[string]uint64{}
	estimatedCost := 1 + customFunctionsCosts[function]

	return &estimatedCost
}

// Resolve resolves the given query against the given unstructured object.
func (cr *CELResolver) Resolve(query string, unstructuredObjectMap map[string]interface{}) map[string]string {
	logger := cr.logger.WithValues("query", query)
	env, err := cr.createEnvironment()
	if err != nil {
		logger.Error(err, "ignoring resolution for query")

		return cr.defaultMapping(query)
	}

	ast, iss := env.Parse(query)
	if iss.Err() != nil {
		logger.Error(fmt.Errorf("error parsing CEL query: %w", iss.Err()), "ignoring resolution for query")

		return cr.defaultMapping(query)
	}

	program, err := cr.compileProgram(env, ast)
	if err != nil {
		logger.Error(err, "ignoring resolution for query")

		return cr.defaultMapping(query)
	}

	out, evalDetails, err := cr.evaluateProgram(program, unstructuredObjectMap)
	logger = cr.addCostLogging(logger, evalDetails)
	if err != nil {
		logger.V(1).Info("ignoring resolution for query", "info", err)

		return cr.defaultMapping(query)
	}

	return cr.processResult(query, out)
}

func (cr *CELResolver) createEnvironment() (*cel.Env, error) {
	return cel.NewEnv(
		cel.CrossTypeNumericComparisons(true),
		cel.DefaultUTCTimeZone(true),
		cel.EagerlyValidateDeclarations(true),
	)
}

func (cr *CELResolver) compileProgram(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
	const costLimit = 1000000

	return env.Program(
		ast,
		cel.CostLimit(costLimit),
		cel.CostTracking(new(costEstimator)),
	)
}

func (cr *CELResolver) evaluateProgram(program cel.Program, obj map[string]interface{}) (ref.Val, *cel.EvalDetails, error) {
	return program.Eval(map[string]interface{}{"o": obj})
}

func (cr *CELResolver) addCostLogging(logger klog.Logger, evalDetails *cel.EvalDetails) klog.Logger {
	logger = logger.WithValues("costLimit", 1000000)
	if evalDetails != nil {
		logger = logger.WithValues("queryCost", *evalDetails.ActualCost())
	}
	logger.V(4).Info("CEL query runtime cost")

	return logger
}

func (cr *CELResolver) processResult(query string, out ref.Val) map[string]string {
	cr.mutex.Lock()
	cr.resolvedFieldParent = query[strings.LastIndex(query, ".")+1:]
	cr.mutex.Unlock()
	switch out.Type() {
	case types.BoolType, types.DoubleType, types.IntType, types.StringType, types.UintType:
		return map[string]string{query: fmt.Sprintf("%v", out.Value())}
	case types.MapType:
		return cr.resolveMap(&out)
	case types.ListType:
		return cr.resolveList(&out)
	case types.NullType:
		return map[string]string{query: "<nil>"}
	default:
		cr.logger.Error(fmt.Errorf("unsupported output type %q", out.Type()), "ignoring resolution for query")

		return cr.defaultMapping(query)
	}
}

func (cr *CELResolver) resolveList(out *ref.Val) map[string]string {
	m := map[string]string{}
	outList, ok := (*out).Value().([]interface{})
	if !ok {
		cr.logger.V(1).Error(errors.New("error casting output to []interface{}"), "ignoring resolution for query")

		return nil
	}
	cr.resolveListInner(outList, m)

	return m
}

func (cr *CELResolver) resolveMap(out *ref.Val) map[string]string {
	m := map[string]string{}
	outMap, ok := (*out).Value().(map[string]interface{})
	if !ok {
		cr.logger.V(1).Error(errors.New("error casting output to map[string]interface{}"), "ignoring resolution for query")

		return nil
	}
	cr.resolveMapInner(outMap, m)

	return m
}

func (cr *CELResolver) resolveListInner(list []interface{}, out map[string]string) {
	for i, v := range list {
		switch v := v.(type) {
		case string, int, uint, float64, bool:
			out[cr.resolvedFieldParent+"#"+strconv.Itoa(i)] = fmt.Sprintf("%v", v)
		case []interface{}:
			cr.resolveListInner(v, out)
		case map[string]interface{}:
			cr.resolveMapInner(v, out)
		default:
			cr.logger.V(1).Error(fmt.Errorf("encountered composite value %q at index %d, skipping", v, i), "ignoring resolution for query")
		}
	}
}

func (cr *CELResolver) resolveMapInner(m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		cr.mutex.Lock()
		cr.resolvedFieldParent = k
		cr.mutex.Unlock()
		switch v := v.(type) {
		case string, int, uint, float64, bool:
			out[k] = fmt.Sprintf("%v", v)
		case []interface{}:
			cr.resolveListInner(v, out)
		case map[string]interface{}:
			cr.resolveMapInner(v, out)
		default:
			cr.logger.V(1).Error(fmt.Errorf("encountered composite value %q at key %q, skipping", v, k), "ignoring resolution for query")
		}
	}
}

func (cr *CELResolver) defaultMapping(query string) map[string]string {
	return map[string]string{query: query}
}
