package completion

import (
	"slices"
	"strings"
)

// Object is a generic struct for completion caching
type Object struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// Objects is a slice of Object, with helper methods for filtering and sorting
type Objects []Object

// Strings returns the names of the objects as a slice of strings, in the same order as the original slice
func (co *Objects) Strings() []string {
	out := make([]string, 0)
	for _, co := range *co {
		out = append(out, co.Name)
	}
	return out
}

// FilterSuggestionsByPartialMatch filters the objects by partial match of the Name field with the toComplete string,
// and returns a new Objects slice sorted by Namespace and Name
func (co *Objects) FilterSuggestionsByPartialMatch(toComplete string) *Objects {
	out := &Objects{}
	if co == nil {
		return out
	}
	for _, p := range *co {
		if strings.HasPrefix(p.Name, toComplete) {
			*out = append(*out, p)
		}
	}
	slices.SortFunc(*out, func(a, b Object) int {
		// Sort by Namespace, then Name
		if a.Namespace != b.Namespace {
			return strings.Compare(a.Namespace, b.Namespace)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// FilterByNamespaces filters the objects by the given namespace, or returns all if allNamespaces is true,
// and returns a new Objects slice
func (co *Objects) FilterByNamespaces(namespace string, allNamespaces bool) *Objects {
	out := &Objects{}
	if co == nil {
		return out
	}
	for _, p := range *co {
		if p.Namespace == namespace || allNamespaces {
			*out = append(*out, p)
		}
	}
	return out
}
