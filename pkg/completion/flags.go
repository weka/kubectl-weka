package completion

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func SuggestFlagsForPositionalArgumentCompletion(cmd *cobra.Command, args []string, toComplete string, flags []string) []string {
	used := map[string]struct{}{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2)
			used[parts[0]] = struct{}{}
		}
	}
	var suggestions []string
	for _, flag := range flags {
		if _, ok := used[flag]; ok {
			continue
		}
		if toComplete == "" || strings.HasPrefix(flag, toComplete) {
			suggestions = append(suggestions, flag)
		}
	}
	return suggestions
}

// SuggestAllUnusedFlagsWithUsageForCompletion returns all unused flags for a command in the format --flag\tUsage or -f, --flag\tUsage, filtered by toComplete prefix.
func SuggestAllUnusedFlagsWithUsageForCompletion(cmd *cobra.Command, args []string, toComplete string) []string {
	used := map[string]struct{}{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2)
			used[parts[0]] = struct{}{}
		}
	}
	var suggestions []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		long := "--" + f.Name
		if _, ok := used[long]; ok {
			return
		}
		// If already set by shorthand, skip
		if f.Shorthand != "" {
			short := "-" + f.Shorthand
			if _, ok := used[short]; ok {
				return
			}
		}
		var flagStr string
		if f.Shorthand != "" {
			flagStr = "-" + f.Shorthand + ", --" + f.Name
		} else {
			flagStr = "--" + f.Name
		}
		if toComplete == "" || strings.HasPrefix(flagStr, toComplete) || strings.HasPrefix(long, toComplete) {
			suggestions = append(suggestions, flagStr+"\t"+f.Usage)
		}
	})
	return suggestions
}
