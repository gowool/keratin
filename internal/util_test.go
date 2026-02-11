package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNegotiateFormat(t *testing.T) {
	tests := []struct {
		name          string
		accepted      []string
		offered       []string
		want          string
		wantPanic     bool
		panicContains string
	}{
		{
			name:     "exact match",
			accepted: []string{"application/json"},
			offered:  []string{"application/json", "text/html"},
			want:     "application/json",
		},
		{
			name:     "first accepted matches second offered",
			accepted: []string{"text/html", "application/json"},
			offered:  []string{"application/json", "text/html"},
			want:     "text/html",
		},
		{
			name:     "empty accepted returns first offered",
			accepted: []string{},
			offered:  []string{"application/json", "text/html"},
			want:     "application/json",
		},
		{
			name:     "no accepted returns empty string",
			accepted: []string{"text/xml"},
			offered:  []string{"application/json", "text/html"},
			want:     "",
		},
		{
			name:     "wildcard in accepted",
			accepted: []string{"*/*"},
			offered:  []string{"application/json", "text/html"},
			want:     "application/json",
		},
		{
			name:     "wildcard in offered",
			accepted: []string{"application/json"},
			offered:  []string{"*/*", "text/html"},
			want:     "*/*",
		},
		{
			name:     "partial match with wildcard at end",
			accepted: []string{"application/*"},
			offered:  []string{"application/json"},
			want:     "application/json",
		},
		{
			name:     "case sensitive match",
			accepted: []string{"Application/JSON"},
			offered:  []string{"application/json"},
			want:     "",
		},
		{
			name:     "empty accepted list",
			accepted: []string{},
			offered:  []string{"application/json"},
			want:     "application/json",
		},
		{
			name:     "multiple accepted matches first",
			accepted: []string{"text/html", "application/json"},
			offered:  []string{"text/html", "application/json"},
			want:     "text/html",
		},
		{
			name:     "complex media types",
			accepted: []string{"application/vnd.api+json"},
			offered:  []string{"application/json", "application/vnd.api+json"},
			want:     "application/vnd.api+json",
		},
		{
			name:          "panic when no offered formats",
			accepted:      []string{"application/json"},
			offered:       []string{},
			wantPanic:     true,
			panicContains: "must provide at least one offer",
		},
		{
			name:          "panic when offered is nil",
			accepted:      []string{"application/json"},
			offered:       nil,
			wantPanic:     true,
			panicContains: "must provide at least one offer",
		},
		{
			name:     "wildcard matches at any position",
			accepted: []string{"text/*"},
			offered:  []string{"text/html"},
			want:     "text/html",
		},
		{
			name:     "charset parameter in accepted",
			accepted: []string{"application/json;charset=utf-8"},
			offered:  []string{"application/json"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				require.Panics(t, func() {
					NegotiateFormat(tt.accepted, tt.offered...)
				})
				return
			}

			got := NegotiateFormat(tt.accepted, tt.offered...)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseAcceptHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty header",
			input: "",
			want:  []string{},
		},
		{
			name:  "single format",
			input: "application/json",
			want:  []string{"application/json"},
		},
		{
			name:  "multiple formats",
			input: "application/json, text/html, text/plain",
			want:  []string{"application/json", "text/html", "text/plain"},
		},
		{
			name:  "formats with quality values",
			input: "application/json;q=0.9, text/html;q=0.8",
			want:  []string{"application/json", "text/html"},
		},
		{
			name:  "formats with charset",
			input: "application/json; charset=utf-8, text/html; charset=utf-8",
			want:  []string{"application/json", "text/html"},
		},
		{
			name:  "formats with multiple parameters",
			input: "application/json; charset=utf-8; q=0.9",
			want:  []string{"application/json"},
		},
		{
			name:  "whitespace handling",
			input: "  application/json  ,  text/html  ",
			want:  []string{"application/json", "text/html"},
		},
		{
			name:  "empty parameters",
			input: "application/json;",
			want:  []string{"application/json"},
		},
		{
			name:  "empty parts after semicolon",
			input: "application/json;,text/html",
			want:  []string{"application/json", "text/html"},
		},
		{
			name:  "complex media types",
			input: "application/vnd.api+json;q=0.9, text/html",
			want:  []string{"application/vnd.api+json", "text/html"},
		},
		{
			name:  "trailing comma",
			input: "application/json,",
			want:  []string{"application/json"},
		},
		{
			name:  "leading comma",
			input: ",application/json",
			want:  []string{"application/json"},
		},
		{
			name:  "only comma",
			input: ",",
			want:  []string{},
		},
		{
			name:  "multiple commas in a row",
			input: "application/json,,,text/html",
			want:  []string{"application/json", "text/html"},
		},
		{
			name:  "wildcard format",
			input: "*/*, application/json",
			want:  []string{"*/*", "application/json"},
		},
		{
			name:  "mixed whitespace and parameters",
			input: "application/json ; q=0.9 , text/html;q=0.8",
			want:  []string{"application/json", "text/html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAcceptHeader(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNegotiateFormat_ParseAcceptHeader_Integration(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		offered      []string
		want         string
	}{
		{
			name:         "browser header negotiation",
			acceptHeader: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
			offered:      []string{"application/json", "text/html"},
			want:         "text/html",
		},
		{
			name:         "API client prefers JSON",
			acceptHeader: "application/json, application/xml;q=0.9, */*;q=0.8",
			offered:      []string{"text/html", "application/json", "application/xml"},
			want:         "application/json",
		},
		{
			name:         "wildcard fallback",
			acceptHeader: "*/*",
			offered:      []string{"application/json"},
			want:         "application/json",
		},
		{
			name:         "no match available",
			acceptHeader: "text/xml",
			offered:      []string{"application/json", "text/html"},
			want:         "",
		},
		{
			name:         "empty accept header",
			acceptHeader: "",
			offered:      []string{"application/json", "text/html"},
			want:         "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accepted := ParseAcceptHeader(tt.acceptHeader)
			got := NegotiateFormat(accepted, tt.offered...)
			require.Equal(t, tt.want, got)
		})
	}
}
