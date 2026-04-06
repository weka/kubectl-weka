package hostcheck

import (
	"testing"

	"github.com/weka/kubectl-weka/pkg/version"
)

// TestVersionCompatibility tests the cache invalidation on version changes
func TestVersionCompatibility(t *testing.T) {
	// Save original version
	originalVersion := version.Version
	defer func() {
		version.Version = originalVersion
	}()

	tests := []struct {
		name           string
		cachedVersion  string
		currentVersion string
		shouldBeValid  bool
		description    string
	}{
		// Same version
		{"same_version", "0.2.1", "0.2.1", true, "Same version should be compatible"},
		{"same_goreleaser", "0.2.5-SNAPSHOT-0d2770f", "0.2.5-SNAPSHOT-0d2770f", true, "Goreleaser format same version"},

		// Patch version updates (same minor)
		{"patch_update", "0.2.1", "0.2.5", true, "Patch update should use cache"},
		{"patch_update_goreleaser", "0.2.1", "0.2.5-SNAPSHOT-0d2770f", true, "Patch update with goreleaser should use cache"},

		// Minor version updates (different minor)
		{"minor_update", "0.2.1", "0.3.0", false, "Minor update should invalidate cache"},
		{"minor_update_to_goreleaser", "0.2.1", "0.3.0-SNAPSHOT-0d2770f", false, "Minor update to goreleaser should invalidate"},

		// Major version updates
		{"major_update", "0.2.1", "1.0.0", false, "Major update should invalidate cache"},
		{"major_update_goreleaser", "1.0.0", "2.0.0-SNAPSHOT", false, "Major update should invalidate cache"},

		// Old cache (empty version) with dev version (should be compatible)
		{"old_cache_with_dev", "", "dev", true, "Old cache with dev version should be compatible"},

		// Old cache (empty version) with production version (should be INVALID)
		{"old_cache_with_production", "", "0.2.1", false, "Old cache with production version should be invalid (upgrading from dev)"},
		{"old_cache_with_goreleaser", "", "0.2.5-SNAPSHOT-0d2770f", false, "Old cache with goreleaser should be invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set current version
			version.Version = tt.currentVersion

			// Test version compatibility
			result := isVersionCompatible(tt.cachedVersion)

			if result != tt.shouldBeValid {
				t.Errorf("%s: expected %v, got %v (cached=%q, current=%q)",
					tt.description, tt.shouldBeValid, result, tt.cachedVersion, tt.currentVersion)
			}
		})
	}
}

// TestVersionInvalidationScenarios tests real-world cache invalidation scenarios
func TestVersionInvalidationScenarios(t *testing.T) {
	originalVersion := version.Version
	defer func() {
		version.Version = originalVersion
	}()

	scenarios := []struct {
		name        string
		upgradePath []string // [old_cached_version, new_version]
		shouldUse   bool     // should cache be used after upgrade?
		scenario    string
	}{
		{
			"patch_to_patch",
			[]string{"0.2.1", "0.2.5"},
			true,
			"User upgrades 0.2.1 → 0.2.5 (patch only), should use cache",
		},
		{
			"dev_to_dev",
			[]string{"dev", "dev"},
			true,
			"Dev version stays dev, should use cache",
		},
		{
			"dev_to_production",
			[]string{"dev", "0.2.1"},
			false,
			"User upgrades from dev to production 0.2.1, should INVALIDATE old cache",
		},
		{
			"production_to_next_minor",
			[]string{"0.2.5", "0.3.0"},
			false,
			"User upgrades 0.2.5 → 0.3.0 (minor), should INVALIDATE cache",
		},
		{
			"production_to_goreleaser",
			[]string{"0.2.5", "0.2.5-SNAPSHOT-0d2770f"},
			true,
			"User updates to goreleaser version same minor, should use cache",
		},
		{
			"goreleaser_to_next_minor",
			[]string{"0.2.5-SNAPSHOT-abcdef", "0.3.0"},
			false,
			"User updates from 0.2.5-SNAPSHOT to 0.3.0, should INVALIDATE cache",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			cachedVersion := scenario.upgradePath[0]
			newVersion := scenario.upgradePath[1]

			version.Version = newVersion
			result := isVersionCompatible(cachedVersion)

			if result != scenario.shouldUse {
				t.Errorf("%s: expected cache valid=%v, got %v",
					scenario.scenario, scenario.shouldUse, result)
			}
		})
	}
}

// TestOldCacheInvalidation verifies that old caches (empty version) are properly invalidated
func TestOldCacheInvalidation(t *testing.T) {
	originalVersion := version.Version
	defer func() {
		version.Version = originalVersion
	}()

	// Scenario: User has old cache from before version tracking (empty PluginVersion)
	// They upgrade from "dev" to production release

	// First with dev: old cache should be valid
	version.Version = "dev"
	if !isVersionCompatible("") {
		t.Error("Old cache should be compatible with dev version")
	}

	// Now upgrade to production: old cache should be INVALID
	version.Version = "0.2.1"
	if isVersionCompatible("") {
		t.Error("Old cache should be INVALID when upgrading from dev to production 0.2.1")
	}

	// Now upgrade to different version: still invalid
	version.Version = "0.3.0"
	if isVersionCompatible("") {
		t.Error("Old cache should be INVALID when at version 0.3.0")
	}

	// But if we cache something at 0.2.1, then patch update, it should be valid
	version.Version = "0.2.5"
	if !isVersionCompatible("0.2.1") {
		t.Error("Cache from 0.2.1 should be valid when current is 0.2.5")
	}
}
