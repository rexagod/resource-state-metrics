package resolver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/klog/v2"
)

func TestNewCELResolver_Resolve(t *testing.T) {
	t.Parallel()
	unstructuredObjectMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "test-namespace",
		},
		"fields": map[string]interface{}{
			"nil":     nil,
			"integer": 1,
			"string":  "bar",
			"array":   [3]string{"a", "b", "c"},
			"slice":   []string{"a", "b", "c"},
			"map": map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			"float":   1.1,
			"rune":    'a',
			"boolean": true,
		},
	}
	tests := []struct {
		name  string
		query string
		want  map[string]string
	}{
		{
			name:  "field exists and is a string",
			query: "o.fields.string",
			want: map[string]string{
				"o.fields.string": "bar",
			},
		},
		{
			name:  "field exists and is an integer",
			query: "o.fields.integer",
			want: map[string]string{
				"o.fields.integer": "1",
			},
		},
		{
			name:  "field exists and is a float",
			query: "o.fields.float",
			want: map[string]string{
				"o.fields.float": "1.1",
			},
		},
		{
			name:  "field exists and is a rune",
			query: "o.fields.rune",
			want: map[string]string{
				"o.fields.rune": "97",
			},
		},
		{
			name:  "field exists and is a boolean",
			query: "o.fields.boolean",
			want: map[string]string{
				"o.fields.boolean": "true",
			},
		},
		{
			name:  "field exists and is an array",
			query: "o.fields.array[1]",
			want: map[string]string{
				"o.fields.array[1]": "b",
			},
		},
		{
			name:  "field exists and is a slice",
			query: "o.fields.slice[1]",
			want: map[string]string{
				"o.fields.slice[1]": "b",
			},
		},
		{
			name:  "field exists and is a map",
			query: "o.fields.map.foo.bar",
			want: map[string]string{
				"o.fields.map.foo.bar": "baz",
			},
		},
		{
			name:  "field exists and is nil",
			query: "o.fields.nil",
			want: map[string]string{
				"o.fields.nil": "<nil>",
			},
		},
		{
			name:  "error traversing obj",
			query: "o.fields.string.bar",
			want: map[string]string{
				"o.fields.string.bar": "o.fields.string.bar",
			},
		},
		{
			name:  "field does not exist",
			query: "o.fields.bar",
			want: map[string]string{
				"o.fields.bar": "o.fields.bar",
			},
		},
		{
			name:  "intermediate field does not exist",
			query: "o.fields.fake.string",
			want: map[string]string{
				"o.fields.fake.string": "o.fields.fake.string",
			},
		},
		{
			name:  "intermediate field is null", // happens easily in YAML
			query: "o.fields.nil.foo",
			want: map[string]string{
				"o.fields.nil.foo": "o.fields.nil.foo",
			},
		},
	}

	cr := NewCELResolver(klog.NewKlogr())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cr.Resolve(tt.query, unstructuredObjectMap); !cmp.Equal(got, tt.want) {
				t.Errorf("%s", cmp.Diff(got, tt.want))
			}
		})
	}
}
