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

/*
This test performs conformance testing for ResourceMetricsMonitor behavior as
seen with the ResourceStateMetrics controller. It does so by testing all golden
rules defined under each resolver's "customresourcestatemetrics_conformance"
directory.

It verifies feature parity with KubeStateMetrics' CustomResourceStateMetrics
feature-set, by deploying a set of golden ResourceMetricsMonitor
configurations, each reflecting an existing KubeStateMetrics'
CustomResourceStateMetrics configuration, and validating that:
* there are no errors, and,
* the expected metrics are emitted with the expected labelsets.

Certain behaviors may differ under the ResourceStateMetrics controller, owing
to them simply making more sense generally, and will be documented in their
respective golden configuration files.
*/

package tests
