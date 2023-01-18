package statefulset

import (
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/merge"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
			if got := merge.LabelSelectorRequirementByKey(tt.args.labelSelectorRequirements, tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLabelSelectorRequirementByKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeSpec(t *testing.T) {

	original := New(
		WithName("original"),
		WithServiceName("original-svc-name"),
		WithReplicas(3),
		WithRevisionHistoryLimit(10),
		WithPodManagementPolicyType(appsv1.OrderedReadyPodManagement),
		WithSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
				"e": "4",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "key-0",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"A", "B", "C"},
				},
				{
					Key:      "key-2",
					Operator: metav1.LabelSelectorOpExists,
					Values:   []string{"F", "D", "E"},
				},
			},
		}),
	)

	override := New(
		WithName("override"),
		WithServiceName("override-svc-name"),
		WithReplicas(5),
		WithRevisionHistoryLimit(15),
		WithPodManagementPolicyType(appsv1.ParallelPodManagement),
		WithSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"a": "10",
				"b": "2",
				"c": "30",
				"d": "40",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "key-0",
					Operator: metav1.LabelSelectorOpDoesNotExist,
					Values:   []string{"Z"},
				},
				{
					Key:      "key-1",
					Operator: metav1.LabelSelectorOpExists,
					Values:   []string{"A", "B", "C", "D"},
				},
			},
		}),
	)

	mergedSpec := merge.StatefulSetSpecs(original.Spec, override.Spec)

	t.Run("Primitive fields of spec have been merged correctly", func(t *testing.T) {
		assert.Equal(t, "override-svc-name", mergedSpec.ServiceName)
		assert.Equal(t, int32(5), *mergedSpec.Replicas)
		assert.Equal(t, int32(15), *mergedSpec.RevisionHistoryLimit)
		assert.Equal(t, appsv1.ParallelPodManagement, mergedSpec.PodManagementPolicy)
	})

	matchLabels := mergedSpec.Selector.MatchLabels
	assert.Len(t, matchLabels, 5)

	t.Run("Match labels have been merged correctly", func(t *testing.T) {
		assert.Equal(t, "10", matchLabels["a"])
		assert.Equal(t, "2", matchLabels["b"])
		assert.Equal(t, "30", matchLabels["c"])
		assert.Equal(t, "40", matchLabels["d"])
		assert.Equal(t, "4", matchLabels["e"])
	})

	t.Run("Test Match Expressions have been merged correctly", func(t *testing.T) {
		matchExpressions := mergedSpec.Selector.MatchExpressions
		assert.Len(t, matchExpressions, 3)
		t.Run("Elements are sorted in alphabetical order", func(t *testing.T) {
			assert.Equal(t, "key-0", matchExpressions[0].Key)
			assert.Equal(t, "key-1", matchExpressions[1].Key)
			assert.Equal(t, "key-2", matchExpressions[2].Key)
		})

		t.Run("Test operator merging", func(t *testing.T) {
			assert.Equal(t, metav1.LabelSelectorOpDoesNotExist, matchExpressions[0].Operator)
			assert.Equal(t, metav1.LabelSelectorOpExists, matchExpressions[1].Operator)
			assert.Equal(t, metav1.LabelSelectorOpExists, matchExpressions[2].Operator)
		})

		t.Run("Test values are merged and sorted", func(t *testing.T) {
			assert.Equal(t, []string{"A", "B", "C", "Z"}, matchExpressions[0].Values)
			assert.Equal(t, []string{"A", "B", "C", "D"}, matchExpressions[1].Values)
			assert.Equal(t, []string{"D", "E", "F"}, matchExpressions[2].Values)
		})
	})
}

func TestMergeSpecLabelSelector(t *testing.T) {
	tests := []struct {
		name     string
		original appsv1.StatefulSet
		override appsv1.StatefulSet
		expected *metav1.LabelSelector
	}{
		{
			name:     "Empty label selectors in both sources",
			original: New(WithName("original")),
			override: New(WithName("override")),
			expected: nil,
		},
		{
			name:     "Empty original label selector",
			original: New(WithName("original")),
			override: New(WithName("override"), WithSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
				},
			})),
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
				},
			},
		},
		{
			name: "Empty override label selector",
			original: New(WithName("original"), WithSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
				},
			})),
			override: New(WithName("override")),
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
				},
			},
		},
		{
			name: "Merge values label selectors from both resources",
			original: New(WithName("original"), WithSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "1",
					"b": "2",
					"c": "3",
					"e": "4",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "key-0",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"A", "B", "C"},
					},
					{
						Key:      "key-2",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"F", "D", "E"},
					},
				},
			})),
			override: New(WithName("override"), WithSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
					"c": "30",
					"d": "40",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "key-0",
						Operator: metav1.LabelSelectorOpDoesNotExist,
						Values:   []string{"Z"},
					},
					{
						Key:      "key-1",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"A", "B", "C", "D"},
					},
				},
			})),
			expected: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"a": "10",
					"b": "2",
					"c": "30",
					"d": "40",
					"e": "4",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "key-0",
						Operator: metav1.LabelSelectorOpDoesNotExist,
						Values:   []string{"A", "B", "C", "Z"},
					},
					{
						Key:      "key-1",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"A", "B", "C", "D"},
					},
					{
						Key:      "key-2",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"D", "E", "F"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergedSpec := merge.StatefulSets(tt.original, tt.override)
			assert.Equal(t, tt.expected, mergedSpec.Spec.Selector)
		})
	}
}
