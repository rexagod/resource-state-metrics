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

package internal

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

// Options represents the command-line Options.
type Options struct {
	AutoGOMAXPROCS  *bool
	RatioGOMEMLIMIT *float64
	Kubeconfig      *string
	MasterURL       *string
	SelfHost        *string
	SelfPort        *int
	MainHost        *string
	MainPort        *int
	TryNoCache      *bool
	Workers         *int
	Version         *bool

	logger klog.Logger
}

// NewOptions returns a new Options.
func NewOptions(logger klog.Logger) *Options {
	return &Options{
		logger: logger,
	}
}

// Read reads the command-line flags and applies overrides, if any.
//
//nolint:lll // Flag descriptions should be, and are, descriptive.
func (o *Options) Read() {
	o.AutoGOMAXPROCS = flag.Bool("auto-gomaxprocs", true, "Automatically set GOMAXPROCS to match CPU quota.")
	o.RatioGOMEMLIMIT = flag.Float64("ratio-gomemlimit", 0.9, "GOMEMLIMIT to memory quota ratio.")
	o.Kubeconfig = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	o.MasterURL = flag.String("master", os.Getenv("KUBERNETES_MASTER"), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	o.SelfHost = flag.String("self-host", "::", "Host to expose self (telemetry) metrics on.")
	o.SelfPort = flag.Int("self-port", 9998, "Port to expose self (telemetry) metrics on.")
	o.MainHost = flag.String("main-host", "::", "Host to expose main metrics on.")
	o.MainPort = flag.Int("main-port", 9999, "Port to expose main metrics on.")
	o.TryNoCache = flag.Bool("try-no-cache", false, "Force the API server to [GET/LIST] the most recent versions.")
	o.Workers = flag.Int("workers", 2, "Number of workers processing the queue.")
	o.Version = flag.Bool("version", false, "Print version information and quit")
	flag.Parse()

	// Respect overrides, this also helps in testing without setting the same defaults in a bunch of places.
	flag.VisitAll(func(f *flag.Flag) {
		// Don't override flags that have been set. Environment variable do not take precedence over command-line flags.
		if f.Value.String() != f.DefValue {
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
