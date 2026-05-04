package main

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/vormadev/vorma/kit/matcher"
)

func TestParseSegments(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		expected []string
	}{
		{"empty path", "", []string{}},
		{"root path", "/", []string{""}},
		{"simple path", "/users", []string{"users"}},
		{"multi-segment path", "/api/v1/users", []string{"api", "v1", "users"}},
		{"trailing slash", "/users/", []string{"users", ""}},
		{
			"path with parameters",
			"/users/:id/posts",
			[]string{"users", ":id", "posts"},
		},
		{
			"path with parameters, implicit index segment",
			"/users/:id/posts/",
			[]string{"users", ":id", "posts", ""},
		},
		{
			"path with parameters, explicit index segment",
			"/users/:id/posts/_index",
			[]string{"users", ":id", "posts", "_index"},
		},
		{"path with splat", "/files/*", []string{"files", "*"}},
		{"multiple slashes", "//api///users", []string{"api", "users"}},
		{
			"complex path",
			"/api/v1/users/:user_id/posts/:post_id/comments",
			[]string{
				"api",
				"v1",
				"users",
				":user_id",
				"posts",
				":post_id",
				"comments",
			},
		},
		{
			"unicode path",
			"/café/über/resumé",
			[]string{"café", "über", "resumé"},
		},
	}

	run_matcher_backends(t, func(t *testing.T, backend matcher_test_backend) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				result := backend.parse_segments(t, tc.path)
				if !reflect.DeepEqual(result, tc.expected) {
					t.Errorf(
						"ParseSegments(%q) = %v, want %v",
						tc.path,
						result,
						tc.expected,
					)
				}
			})
		}
	})
}

func BenchmarkParseSegments(b *testing.B) {
	paths := []string{
		"/",
		"/api/v1/users",
		"/api/v1/users/123/posts/456/comments",
		"/files/documents/reports/quarterly/q3-2023.pdf",
	}

	b.Run("ParseSegments", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			segments := matcher.ParseSegments(paths[i%len(paths)])
			runtime.KeepAlive(segments)
		}
	})
}
