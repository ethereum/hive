package libhive

import (
	"reflect"
	"slices"
	"testing"
)

func TestRedactCommandArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "separate value argument",
			in:   []string{"hive", "--sim", "ethereum/eels/consume-engine", "--sim.buildarg", "github_token=ghs_abc123"},
			want: []string{"hive", "--sim", "ethereum/eels/consume-engine", "--sim.buildarg", "github_token=<redacted>"},
		},
		{
			name: "inline value argument",
			in:   []string{"hive", "--sim.buildarg=github_token=ghs_abc123"},
			want: []string{"hive", "--sim.buildarg=github_token=<redacted>"},
		},
		{
			name: "single dash and uppercase key",
			in:   []string{"hive", "-sim.buildarg", "GITHUB_TOKEN=ghs_abc123"},
			want: []string{"hive", "-sim.buildarg", "GITHUB_TOKEN=<redacted>"},
		},
		{
			name: "non-sensitive build args are kept, including values containing '='",
			in:   []string{"hive", "--sim.buildarg", "fixtures=https://example.org/f.tar.gz?a=b", "--sim.buildarg", "branch=forks/amsterdam"},
			want: []string{"hive", "--sim.buildarg", "fixtures=https://example.org/f.tar.gz?a=b", "--sim.buildarg", "branch=forks/amsterdam"},
		},
		{
			name: "mixed sensitive and non-sensitive",
			in:   []string{"hive", "--sim.buildarg", "branch=main", "--sim.buildarg", "github_token=x", "--client", "go-ethereum"},
			want: []string{"hive", "--sim.buildarg", "branch=main", "--sim.buildarg", "github_token=<redacted>", "--client", "go-ethereum"},
		},
		{
			name: "other flags whose values look like KEY=VALUE are untouched",
			in:   []string{"hive", "--sim.limit", "github_token=1", "--client", "go-ethereum"},
			want: []string{"hive", "--sim.limit", "github_token=1", "--client", "go-ethereum"},
		},
		{
			name: "trailing buildarg flag without a value",
			in:   []string{"hive", "--sim.buildarg"},
			want: []string{"hive", "--sim.buildarg"},
		},
		{
			name: "empty",
			in:   []string{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := slices.Clone(tt.in)
			got := redactCommandArgs(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("redactCommandArgs(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
			if !slices.Equal(tt.in, orig) {
				t.Errorf("input slice was modified: %q", tt.in)
			}
		})
	}
}

func TestFilterClientDesignatorsIsCaseInsensitive(t *testing.T) {
	in := []ClientDesignator{{
		Client:        "go-ethereum",
		Nametag:       "default",
		DockerfileExt: "git",
		BuildArgs: map[string]string{
			"github":       "ethereum/go-ethereum",
			"tag":          "master",
			"github_token": "ghs_abc123",
			"GOPROXY":      "https://proxy.example.org",
		},
	}}
	got := filterClientDesignators(in)
	want := map[string]string{"github": "ethereum/go-ethereum", "tag": "master"}
	if len(got) != 1 || !reflect.DeepEqual(got[0].BuildArgs, want) {
		t.Errorf("filtered build args = %v, want %v", got[0].BuildArgs, want)
	}
	if in[0].BuildArgs["github_token"] != "ghs_abc123" {
		t.Errorf("input designator was modified")
	}
}
