package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSerdeJSONEscapesOnlyJSONSyntax(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"<&>\u2028\u2029", "\"<&>\u2028\u2029\""},
		{`\u2028\u003c`, `"\\u2028\\u003c"`},
		{"\n\r\t\x00\"\\", `"\n\r\t\u0000\"\\"`},
		{"\\\u2028", "\"\\\\\u2028\""},
	} {
		got, err := QuoteJSONString(tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%q: %q, %v; want %q", tc.value, got, err, tc.want)
		}
	}
	if _, err := QuoteJSONString(string([]byte{255})); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestNumericMapsUseRustBTreeOrderingForCLIJSON(t *testing.T) {
	data, err := SerdeJSON(Map[uint16, uint16]{443: 20443, 80: 20080, 22: 20022})
	if err != nil || string(data) != `{"22":20022,"80":20080,"443":20443}` {
		t.Fatal(string(data), err)
	}
}

func TestRequestCodecsRejectCaseAliasDuplicateAndNull(t *testing.T) {
	var stop StopOptions
	if err := json.Unmarshal([]byte(`{"Force":true}`), &stop); err != nil || stop.Force {
		t.Fatal("wrong-case force changed the stop policy", stop, err)
	}
	if err := json.Unmarshal([]byte(`{"force":null}`), &stop); err == nil {
		t.Fatal("null force accepted")
	}
	var update VolumeUpdate
	if err := json.Unmarshal([]byte(`{"retain":false,"retain":true}`), &update); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
	var spec VolumeSpec
	if err := json.Unmarshal([]byte(`{"id":"disk-g1","generation":1,"size_mib":1,"retain":null}`), &spec); err == nil {
		t.Fatal("null retain accepted")
	}
	var activation ActivationRequest
	if err := json.Unmarshal([]byte(`{"Phase":"mutations","fence_id":"f","previous":"p","catalog_digest":"c","plan_digest":"d"}`), &activation); err == nil || activation.Phase == "mutations" {
		t.Fatal("wrong-case phase was accepted", activation, err)
	}
}
