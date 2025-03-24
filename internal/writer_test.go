package internal

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
)

func TestMetricsWriter_writeAllTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		m        metricsWriter
		expected string
	}{
		{
			name:     "empty store",
			m:        metricsWriter{},
			expected: "",
		},
		{
			name: "non-empty store with same number of headers and metrics",
			m: metricsWriter{
				stores: []*StoreType{
					{
						headers: []string{"header1", "header2"},
						metrics: map[types.UID][]string{
							"uid1": {"metric1", "metric2"},
							"uid2": {"metric1", "metric2"},
						},
					},
				},
			},
			expected: "header1\nmetric1metric1header2\nmetric2metric2",
		},
		{
			name: "non-empty store with more number of headers than metrics",
			m: metricsWriter{
				stores: []*StoreType{
					{
						headers: []string{"header1", "header2", "header3"},
						metrics: map[types.UID][]string{
							"uid1": {"metric1", "metric2"},
							"uid2": {"metric1", "metric2", "metric3"},
						},
					},
				},
			},
			expected: "header1\nmetric1metric1header2\nmetric2metric2header3\nmetric3",
		},
		{
			name: "non-empty store with less number of headers than metrics",
			m: metricsWriter{
				stores: []*StoreType{
					{
						headers: []string{"header1"},
						metrics: map[types.UID][]string{
							"uid1": {"metric1", "metric2"},
							"uid2": {"metric1", "metric2"},
						},
					},
				},
			},
			expected: "header1\nmetric1metric1",
		},
		{
			name: "non-empty store with no headers",
			m: metricsWriter{
				stores: []*StoreType{
					{
						headers: []string{},
						metrics: map[types.UID][]string{
							"uid1": {"metric1", "metric1"},
							"uid2": {"metric1"},
						},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &bytes.Buffer{}
			if err := tt.m.writeStores(w); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := w.String(); got != tt.expected {
				t.Fatalf("%s", cmp.Diff(got, tt.expected))
			}
		})
	}
}
