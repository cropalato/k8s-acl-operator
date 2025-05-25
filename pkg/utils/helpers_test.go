/*
Copyright 2024.

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

package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
)

func TestNamespaceMatches(t *testing.T) {
	tests := []struct {
		name      string
		namespace *corev1.Namespace
		selector  rbacoperatorv1.NamespaceSelector
		expected  bool
		expectErr bool
	}{
		{
			name: "matches regex",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dev-app-1",
				},
			},
			selector: rbacoperatorv1.NamespaceSelector{
				NameRegex: GetStringPtr("^dev-.*"),
			},
			expected: true,
		},
		{
			name: "does not match regex",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-app-1",
				},
			},
			selector: rbacoperatorv1.NamespaceSelector{
				NameRegex: GetStringPtr("^dev-.*"),
			},
			expected: false,
		},
		{
			name: "matches annotation",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
					Annotations: map[string]string{
						"team": "platform",
					},
				},
			},
			selector: rbacoperatorv1.NamespaceSelector{
				Annotations: map[string]string{
					"team": "platform",
				},
			},
			expected: true,
		},
		{
			name: "excluded namespace",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dev-system",
				},
			},
			selector: rbacoperatorv1.NamespaceSelector{
				NameRegex:         GetStringPtr("^dev-.*"),
				ExcludeNamespaces: []string{"dev-system"},
			},
			expected: false,
		},
		{
			name: "included namespace",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "special-ns",
				},
			},
			selector: rbacoperatorv1.NamespaceSelector{
				IncludeNamespaces: []string{"special-ns", "another-ns"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NamespaceMatches(tt.namespace, tt.selector)

			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestSliceContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "contains item",
			slice:    []string{"a", "b", "c"},
			item:     "b",
			expected: true,
		},
		{
			name:     "does not contain item",
			slice:    []string{"a", "b", "c"},
			item:     "d",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SliceContains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestUniqueSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "removes duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UniqueSlice(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d but got %d", len(tt.expected), len(result))
				return
			}

			// Check if all expected items are in result
			for _, expected := range tt.expected {
				if !SliceContains(result, expected) {
					t.Errorf("expected item %s not found in result", expected)
				}
			}
		})
	}
}

func TestMapContainsAll(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]string
		required map[string]string
		expected bool
	}{
		{
			name:     "contains all",
			target:   map[string]string{"a": "1", "b": "2", "c": "3"},
			required: map[string]string{"a": "1", "b": "2"},
			expected: true,
		},
		{
			name:     "missing key",
			target:   map[string]string{"a": "1"},
			required: map[string]string{"a": "1", "b": "2"},
			expected: false,
		},
		{
			name:     "wrong value",
			target:   map[string]string{"a": "1", "b": "wrong"},
			required: map[string]string{"a": "1", "b": "2"},
			expected: false,
		},
		{
			name:     "nil required",
			target:   map[string]string{"a": "1"},
			required: nil,
			expected: true,
		},
		{
			name:     "nil target",
			target:   nil,
			required: map[string]string{"a": "1"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapContainsAll(tt.target, tt.required)
			if result != tt.expected {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestMergeMaps(t *testing.T) {
	map1 := map[string]string{"a": "1", "b": "2"}
	map2 := map[string]string{"b": "override", "c": "3"}

	result := MergeMaps(map1, map2)

	expected := map[string]string{"a": "1", "b": "override", "c": "3"}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Expected %s=%s, got %s", k, v, result[k])
		}
	}
}

func TestRemoveFromSlice(t *testing.T) {
	input := []string{"a", "b", "c", "b", "d"}
	result := RemoveFromSlice(input, "b")

	expected := []string{"a", "c", "d"}

	if len(result) != len(expected) {
		t.Errorf("Expected length %d, got %d", len(expected), len(result))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %s at index %d, got %s", v, i, result[i])
		}
	}
}

func TestCopyMap(t *testing.T) {
	original := map[string]string{"a": "1", "b": "2"}
	copy := CopyMap(original)

	// Verify copy is equal
	for k, v := range original {
		if copy[k] != v {
			t.Errorf("Copy missing key %s or wrong value", k)
		}
	}

	// Verify it's a separate map
	copy["c"] = "3"
	if _, exists := original["c"]; exists {
		t.Errorf("Original map was modified")
	}

	// Test nil map
	nilCopy := CopyMap(nil)
	if nilCopy != nil {
		t.Errorf("Expected nil copy of nil map")
	}
}
