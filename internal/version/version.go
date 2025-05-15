/*
Copyright 2025 The Kubernetes resource-state-metrics Authors.

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

// Package version prints the version metadata of the binary.
package version

import (
	"strings"

	"github.com/prometheus/common/version"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// name is an implicit type to allow attaching convenience methods on the controller name.
// All methods return string values and not `name` to avoid exporting this.
type name string

// ControllerName is used in metrics as is, so snake-case is necessary.
var ControllerName name = "resource-state-metrics"

// String returns the controller name as a string.
func (n name) String() string {
	return string(n)
}

// ToPascalCase returns the controller name in PascalCase.
func (n name) ToPascalCase() string {
	return strings.ReplaceAll(cases.Title(language.English, cases.NoLower).String(n.String()), "-", "")
}

// ToSnakeCase returns the controller name in snake_case.
func (n name) ToSnakeCase() string {
	return strings.ReplaceAll(strings.ToLower(n.String()), "-", "_")
}

// Version returns the version metadata of the binary.
func Version() string {
	return version.Print(ControllerName.String())
}
