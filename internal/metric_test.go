package internal

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWriteMetricTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		resolvedLabelKeys   []string
		resolvedLabelValues []string
		expected            string
		wantErr             bool
	}{
		{
			name:                "empty label keys and values",
			resolvedLabelKeys:   []string{},
			resolvedLabelValues: []string{},
			expected:            "{group=\"group\",version=\"version\",kind=\"kind\"} 42.000000\n",
		},
		{
			name:                "multiple label keys and values",
			resolvedLabelKeys:   []string{"key1", "key2"},
			resolvedLabelValues: []string{"value1", "value2"},
			expected:            "{key1=\"value1\",key2=\"value2\",group=\"group\",version=\"version\",kind=\"kind\"} 42.000000\n",
		},
		{
			name:                "escaped label values",
			resolvedLabelKeys:   []string{"key1"},
			resolvedLabelValues: []string{"value1\nvalue2"},
			expected:            "{key1=\"value1\\nvalue2\",group=\"group\",version=\"version\",kind=\"kind\"} 42.000000\n",
		},
		{
			name:                "len(keys) < len(values)",
			resolvedLabelKeys:   []string{"key1", "key2"},
			resolvedLabelValues: []string{"value1", "value2", "value3"},
			expected:            "",
			wantErr:             true,
		},
		{
			name:                "len(keys) > len(values)",
			resolvedLabelKeys:   []string{"key1", "key2", "key3"},
			resolvedLabelValues: []string{"value1", "value2"},
			expected:            "",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var writer strings.Builder
			if err := writeMetricTo(&writer, "group", "version", "kind", 42, tt.resolvedLabelKeys, tt.resolvedLabelValues); err != nil && !tt.wantErr {
				t.Fatal(err)
			}
			if got := writer.String(); got != tt.expected {
				t.Errorf("%s", cmp.Diff(got, tt.expected))
			}
		})
	}
}
