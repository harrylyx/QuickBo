package app

import (
	"slices"
	"strings"
	"testing"
)

// Exported rules import back unchanged, unsaved edits included.
func TestRulesExportImportRoundTrip(t *testing.T) {
	a := New()
	a.settingsPath = ""
	a.homeRules = []homeRule{
		{ID: "movie", Name: "电影", Keywords: []string{"电影"}},
		{ID: "custom-1", Name: "纪录片", Keywords: []string{"纪录"}},
	}
	a.resetRuleEdits()
	a.ruleWords[1] = "纪录、纪实" // typed, not saved yet

	data, err := encodeRules(a.draftRules())
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "纪录片" || !slices.Equal(got[1].Keywords, []string{"纪录", "纪实"}) {
		t.Fatalf("round trip: %+v", got)
	}

	b := New()
	b.settingsPath = ""
	b.importRules(got)
	if !b.rulesDirty || len(b.ruleNames) != 2 || b.ruleWords[1] != "纪录、纪实" {
		t.Fatalf("import must land as unsaved edits: dirty=%v names=%v words=%v", b.rulesDirty, b.ruleNames, b.ruleWords)
	}
}

func TestDecodeRulesFormatsAndErrors(t *testing.T) {
	got, err := decodeRules([]byte(`[{"id":"a","name":"甲","keywords":["x"," x ","y"]}]`))
	if err != nil || len(got) != 1 || !slices.Equal(got[0].Keywords, []string{"x", "y"}) {
		t.Fatalf("bare array: %+v %v", got, err)
	}
	for name, in := range map[string]string{
		"not json":   "hello",
		"broken":     `{"rules": [`,
		"no rows":    `{"rules": []}`,
		"no names":   `{"rules": [{"id": "a"}]}`,
		"newer file": `{"version": 99, "rules": [{"id": "a", "name": "甲"}]}`,
	} {
		if _, err := decodeRules([]byte(in)); err == nil {
			t.Errorf("%s: must fail", name)
		}
	}
	_, err = decodeRules([]byte(`{"version": 99, "rules": []}`))
	if err == nil || !strings.Contains(err.Error(), "升级") {
		t.Fatalf("newer file must ask for an upgrade: %v", err)
	}
}
