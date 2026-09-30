package ctl

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRustControllerHTTPFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/http-rust.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Revision string
		Cases    []struct {
			Method, Path      string
			Input             *string
			Headers           map[string]string
			APIKey            *string `json:"api_key"`
			Status            int
			ContentType       *string `json:"content_type"`
			Allow, CSP, Frame *string
			Body              string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Revision != "1ad83a9feed5fac820d74b5e7ef752e50a552740" {
		t.Fatal("wrong oracle")
	}
	value := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Method+" "+tc.Path, func(t *testing.T) {
			s := newServer(testDB(t), tc.APIKey)
			input := value(tc.Input)
			request := httptest.NewRequest(tc.Method, tc.Path, strings.NewReader(input))
			for key, val := range tc.Headers {
				request.Header.Set(key, val)
			}
			response := httptest.NewRecorder()
			s.Handler().ServeHTTP(response, request)
			s.workers.Wait()
			body := response.Body.String()
			if tc.Body == "<PINNED_DASHBOARD>" {
				if !bytes.Equal(response.Body.Bytes(), dashboard) {
					t.Fatal("dashboard changed")
				}
				body = tc.Body
			}
			if response.Code != tc.Status || response.Header().Get("Content-Type") != value(tc.ContentType) || response.Header().Get("Allow") != value(tc.Allow) || response.Header().Get("Content-Security-Policy") != value(tc.CSP) || response.Header().Get("X-Frame-Options") != value(tc.Frame) || body != tc.Body {
				t.Fatalf("got %d %v %q\nwant %d type=%q allow=%q body=%q", response.Code, response.Header(), body, tc.Status, value(tc.ContentType), value(tc.Allow), tc.Body)
			}
		})
	}
}
func TestDashboardMatchesPinnedRustAsset(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256(dashboard))
	if digest != "d88d6227bb3cab8988843bc0da098f92303043b053295b4b1c9c5090b319fb96" {
		t.Fatal("dashboard differs from pinned Rust source", digest)
	}
}
