package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpListsTools(t *testing.T) {
	root := Root()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "ua-refresh") {
		t.Fatalf("help should list ua-refresh, got:\n%s", out.String())
	}
}
