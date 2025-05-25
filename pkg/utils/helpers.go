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

// Package utils provides helper functions for namespace matching and data manipulation.
// It contains the core logic for evaluating namespace selectors and various
// utility functions for working with slices, maps, and pointers.
package utils

import (
	"regexp"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	corev1 "k8s.io/api/core/v1"
)

// NamespaceMatches determines if a namespace matches the given selector criteria.
// It evaluates multiple criteria using AND logic (all must pass):
// 1. Exclusion list (takes precedence - if namespace is excluded, returns false)
// 2. Inclusion list (if specified, namespace must be in the list)
// 3. Name regex pattern (namespace name must match regex)
// 4. Required annotations (all specified annotations must exist with exact values)
// 5. Required labels (all specified labels must exist with exact values)
//
// Returns true only if ALL applicable criteria pass.
func NamespaceMatches(ns *corev1.Namespace, selector rbacoperatorv1.NamespaceSelector) (bool, error) {
	// Check explicit exclusions first
	for _, excluded := range selector.ExcludeNamespaces {
		if ns.Name == excluded {
			return false, nil
		}
	}

	// If include list is specified, namespace must be in it
	if len(selector.IncludeNamespaces) > 0 {
		found := false
		for _, included := range selector.IncludeNamespaces {
			if ns.Name == included {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	// Check name regex
	if selector.NameRegex != nil && *selector.NameRegex != "" {
		matched, err := regexp.MatchString(*selector.NameRegex, ns.Name)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}

	// Check required annotations
	if selector.Annotations != nil {
		if ns.Annotations == nil {
			return false, nil
		}
		for key, value := range selector.Annotations {
			if nsValue, exists := ns.Annotations[key]; !exists || nsValue != value {
				return false, nil
			}
		}
	}

	// Check required labels
	if selector.Labels != nil {
		if ns.Labels == nil {
			return false, nil
		}
		for key, value := range selector.Labels {
			if nsValue, exists := ns.Labels[key]; !exists || nsValue != value {
				return false, nil
			}
		}
	}

	return true, nil
}

// GetStringPtr returns a pointer to the given string
func GetStringPtr(s string) *string {
	return &s
}

// GetBoolPtr returns a pointer to the given bool
func GetBoolPtr(b bool) *bool {
	return &b
}

// GetInt32Ptr returns a pointer to the given int32
func GetInt32Ptr(i int32) *int32 {
	return &i
}

// StringPtrValue returns the value of a string pointer or empty string if nil
func StringPtrValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// BoolPtrValue returns the value of a bool pointer or false if nil
func BoolPtrValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

// Int32PtrValue returns the value of an int32 pointer or 0 if nil
func Int32PtrValue(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

// MergeMaps merges two string maps, with values from the second map taking precedence.
// Used for combining template labels/annotations with default values.
func MergeMaps(map1, map2 map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy from first map
	for k, v := range map1 {
		result[k] = v
	}

	// Override with second map
	for k, v := range map2 {
		result[k] = v
	}

	return result
}

// CopyMap creates a deep copy of a string map.
// Returns nil if the original map is nil.
func CopyMap(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range original {
		result[k] = v
	}

	return result
}

// MapContainsAll checks if the target map contains all key-value pairs from the required map.
// Returns true if all required entries exist with matching values in the target map.
// Used for namespace annotation/label matching.
func MapContainsAll(target, required map[string]string) bool {
	if required == nil {
		return true
	}
	if target == nil {
		return len(required) == 0
	}

	for key, value := range required {
		if targetValue, exists := target[key]; !exists || targetValue != value {
			return false
		}
	}

	return true
}

// SliceContains checks if a string slice contains the given string.
// Performs linear search for exact string matches.
func SliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveFromSlice removes all occurrences of an item from a string slice.
// Returns a new slice with the specified item filtered out.
func RemoveFromSlice(slice []string, item string) []string {
	result := make([]string, 0)
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// UniqueSlice returns a new slice with duplicate strings removed.
// Preserves order of first occurrence of each unique string.
func UniqueSlice(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
