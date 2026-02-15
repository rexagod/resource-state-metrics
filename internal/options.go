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

package internal

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

const (
	autoGOMAXPROCSFlagName  = "auto-gomaxprocs"
	celCostLimitFlagName    = "cel-cost-limit"
	celTimeoutFlagName      = "cel-timeout-seconds"
	kubeconfigFlagName      = "kubeconfig"
	mainHostFlagName        = "main-host"
	mainPortFlagName        = "main-port"
	masterURLFlagName       = "master"
	ratioGOMEMLIMITFlagName = "ratio-gomemlimit"
	selfHostFlagName        = "self-host"
	selfPortFlagName        = "self-port"
	versionFlagName         = "version"
	workersFlagName         = "workers"
)

// Options represents the command-line Options.
type Options struct {
	AutoGOMAXPROCS  *bool
	CELCostLimit    *uint64
	CELTimeout      *int
	Kubeconfig      *string
	MainHost        *string
	MainPort        *int
	MasterURL       *string
	RatioGOMEMLIMIT *float64
	SelfHost        *string
	SelfPort        *int
	Version         *bool
	Workers         *int

	logger klog.Logger
}

// NewOptions returns a new Options.
func NewOptions(logger klog.Logger) *Options {
	return &Options{
		logger: logger,
	}
}

// Read reads the command-line flags and applies overrides, if any.
func (o *Options) Read() {
	o.AutoGOMAXPROCS = flag.Bool(autoGOMAXPROCSFlagName, true, "Automatically set GOMAXPROCS to match CPU quota.")
	o.CELCostLimit = flag.Uint64(celCostLimitFlagName, 10e5, "Maximum cost budget for CEL expression evaluation. CEL cost represents computational complexity: traversing an object field costs 1, invoking a function varies by complexity. This limit prevents runaway expressions from consuming excessive resources. Typical queries cost 100-10000; increase if legitimate queries hit the limit.")
	o.CELTimeout = flag.Int(celTimeoutFlagName, 5, "Maximum time in seconds for CEL expression evaluation. This timeout enforces a wall-clock limit on query execution to prevent slow expressions from blocking metric generation. Increase if complex legitimate queries timeout.")
	o.Kubeconfig = flag.String(kubeconfigFlagName, os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	o.MainHost = flag.String(mainHostFlagName, "::", "Host to expose main metrics on.")
	o.MainPort = flag.Int(mainPortFlagName, 9999, "Port to expose main metrics on.")
	o.MasterURL = flag.String(masterURLFlagName, os.Getenv("KUBERNETES_MASTER"), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	o.RatioGOMEMLIMIT = flag.Float64(ratioGOMEMLIMITFlagName, 0.9, "GOMEMLIMIT to memory quota ratio.")
	o.SelfHost = flag.String(selfHostFlagName, "::", "Host to expose self (telemetry) metrics on.")
	o.SelfPort = flag.Int(selfPortFlagName, 9998, "Port to expose self (telemetry) metrics on.")
	o.Version = flag.Bool(versionFlagName, false, "Print version information and quit")
	o.Workers = flag.Int(workersFlagName, 2, "Number of workers processing managed resources in the workqueue.")
	flag.Parse()

	// Respect overrides, this also helps in testing without setting the same defaults in a bunch of places.
	flag.VisitAll(func(f *flag.Flag) {
		// Don't override flags that have been set. Environment variables do not take precedence over command-line flags.
		if f.Value.String() != f.DefValue {
			o.validateFlag(f.Name, f.Value.String())
			return
		}
		name := f.Name
		overriderForOptionName := `RSM_` + strings.ReplaceAll(strings.ToUpper(name), "-", "_")
		if value, ok := os.LookupEnv(overriderForOptionName); ok {
			o.logger.V(1).Info(fmt.Sprintf("Overriding flag %s with %s=%s", name, overriderForOptionName, value))
			err := flag.Set(name, value)
			if err != nil {
				panic(fmt.Sprintf("Failed to set flag %s to %s: %v", name, value, err))
			}
		}
	})
}

// TODO
func (o *Options) validateFlag(name, value string) error {
	switch name {
	case celTimeoutFlagName:
		valueInt, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: %v", name, err)
		}
		if valueInt <= 0 || valueInt > 300 {
			return fmt.Errorf("%s must be between 1 and 300 seconds", name)
		}
	}

	return nil
}
