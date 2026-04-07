package completion

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"
)

// FetchGithubReleaseVersions fetches all public release versions from a GitHub repo, strips leading 'v', and returns them sorted.
func FetchGithubReleaseVersions(repo string, toComplete string) ([]string, error) {
	type githubRelease struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	client := &http.Client{Timeout: 5 * time.Second}
	url := "https://api.github.com/repos/" + repo + "/releases?per_page=100"
	var releases []githubRelease
	for url != "" {
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var pageReleases []githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&pageReleases); err != nil {
			return nil, err
		}
		releases = append(releases, pageReleases...)
		// Parse Link header for next page
		url = ""
		if link := resp.Header.Get("Link"); link != "" {
			for _, part := range strings.Split(link, ",") {
				sections := strings.Split(strings.TrimSpace(part), ";")
				if len(sections) < 2 {
					continue
				}
				if strings.Contains(sections[1], "rel=\"next\"") {
					nextURL := strings.Trim(sections[0], " <>")
					url = nextURL
					break
				}
			}
		}
	}
	var versions []string
	for _, rel := range releases {
		if rel.Draft || rel.Prerelease {
			continue
		}
		ver := rel.TagName
		if strings.HasPrefix(ver, "v") {
			ver = ver[1:]
		}
		if strings.HasPrefix(ver, toComplete) {
			versions = append(versions, ver)
		}
	}
	slices.Sort(versions)
	return versions, nil
}
