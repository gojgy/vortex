package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeRequestPath(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "EmptyString",
			input: "",
			want:  "/",
		},
		{
			name:  "RootPath",
			input: "/",
			want:  "/",
		},
		{
			name:  "SimplePathWithoutSlashes",
			input: "api",
			want:  "/api/",
		},
		{
			name:  "PathWithLeadingSlash",
			input: "/api",
			want:  "/api/",
		},
		{
			name:  "PathWithTrailingSlash",
			input: "api/",
			want:  "/api/",
		},
		{
			name:  "PathWithBothSlashes",
			input: "/api/",
			want:  "/api/",
		},
		{
			name:  "LongPathWithoutSlashes",
			input: "api/v1/users",
			want:  "/api/v1/users/",
		},
		{
			name:  "LongPathWithLeadingSlash",
			input: "/api/v1/users",
			want:  "/api/v1/users/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeRequestPath(tc.input)
			assert.Equal(t, tc.want, got, "For input: %s", tc.input)
		})
	}
}

func TestGetHostFromURL(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ValidHttpURL",
			input: "http://example.com/path",
			want:  "example.com",
		},
		{
			name:  "ValidHttpsURLWithPort",
			input: "https://localhost:8080/api/v1",
			want:  "localhost:8080",
		},
		{
			name:  "URLWithIPAddress",
			input: "http://127.0.0.1/resource",
			want:  "127.0.0.1",
		},
		{
			name:  "UpstreamNameWithoutPort",
			input: "http://my-backend-service",
			want:  "my-backend-service",
		},
		{
			name:  "URLWithoutPath",
			input: "https://service.internal:3000",
			want:  "service.internal:3000",
		},
		{
			name:  "MalformedURL",
			input: "://broken-url",
			want:  "",
		},
		{
			name:  "EmptyString",
			input: "",
			want:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := GetHostFromURL(tc.input)
			assert.Equal(t, tc.want, got, "For input: %s", tc.input)
		})
	}
}

func TestContains(t *testing.T) {
	t.Run("StringSlice", func(t *testing.T) {
		stringSlice := []string{"apple", "banana", "cherry"}
		testCases := []struct {
			name   string
			target string
			want   bool
		}{
			{name: "ElementExists", target: "banana", want: true},
			{name: "ElementDoesNotExist", target: "grape", want: false},
			{name: "FirstElement", target: "apple", want: true},
			{name: "LastElement", target: "cherry", want: true},
			{name: "EmptyStringTarget", target: "", want: false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got := Contains(tc.target, stringSlice)
				assert.Equal(t, tc.want, got)
			})
		}

		t.Run("EmptySlice", func(t *testing.T) {
			assert.False(t, Contains("any", []string{}))
		})
	})

	t.Run("IntSlice", func(t *testing.T) {
		intSlice := []int{10, 20, 30}
		testCases := []struct {
			name   string
			target int
			want   bool
		}{
			{name: "ElementExists", target: 20, want: true},
			{name: "ElementDoesNotExist", target: 25, want: false},
			{name: "ZeroValueTarget", target: 0, want: false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				got := Contains(tc.target, intSlice)
				assert.Equal(t, tc.want, got)
			})
		}
	})
}
