package statefulset

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergeDistinct(t *testing.T) {
	type args struct {
		original []string
		override []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Does not include duplicate entries",
			args: args{
				original: []string{"a", "b", "c"},
				override: []string{"a", "c"},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "Adds elements from override",
			args: args{
				original: []string{"a", "b", "c"},
				override: []string{"a", "b", "c", "d", "e"},
			},
			want: []string{"a", "b", "c", "d", "e"},
		},
		{
			name: "Doesn't panic with nil input",
			args: args{
				original: nil,
				override: nil,
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeDistinct(tt.args.original, tt.args.override); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeDistinct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLabelSelectorRequirementByKey(t *testing.T) {
	type args struct {
		labelSelectorRequirements []metav1.LabelSelectorRequirement
		key                       string
	}
	tests := []struct {
		name string
		args args
		want *metav1.LabelSelectorRequirement
	}{
		{
			name: "Returns nil if the element is not present",
			args: args{
				labelSelectorRequirements: []metav1.LabelSelectorRequirement{
					{
						Key: "test-key",
					},
				},
				key: "not-found",
			},
			want: nil,
		},
		{
			name: "Finds the element if the key matches an element present.",
			args: args{
				labelSelectorRequirements: []metav1.LabelSelectorRequirement{
					{
						Key: "test-key",
					},
				},
				key: "test-key",
			},
			want: &metav1.LabelSelectorRequirement{

				Key: "test-key",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLabelSelectorRequirementByKey(tt.args.labelSelectorRequirements, tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLabelSelectorRequirementByKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
