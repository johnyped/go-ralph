package herdr

import (
	"encoding/json"
	"testing"
)

func TestAgentCloseByNameParsesAgents(t *testing.T) {
	raw := `{"result":{"agents":[{"name":"ralph-001","pane_id":"w1-2"},{"name":"ralph-002","pane_id":"w1-3"}],"type":"agent_list"}}`
	var resp struct {
		Result struct {
			Agents []struct {
				Name   string `json:"name"`
				PaneID string `json:"pane_id"`
			} `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Result.Agents) != 2 {
		t.Fatalf("want 2 agents, got %d", len(resp.Result.Agents))
	}
	if resp.Result.Agents[0].Name != "ralph-001" || resp.Result.Agents[0].PaneID != "w1-2" {
		t.Errorf("agent[0] = %+v", resp.Result.Agents[0])
	}
}


func TestWorkspaceCreateParsesID(t *testing.T) {
	// Verify the JSON shape we expect from herdr workspace create
	raw := `{"result":{"workspace":{"workspace_id":"3"},"tab":{"tab_id":"3:1"},"root_pane":{"pane_id":"3-1"}}}`
	var resp struct {
		Result struct {
			Workspace struct{ WorkspaceID string `json:"workspace_id"` } `json:"workspace"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.Workspace.WorkspaceID != "3" {
		t.Errorf("workspace_id = %q, want 3", resp.Result.Workspace.WorkspaceID)
	}
}

func TestAgentStartParsesID(t *testing.T) {
	raw := `{"result":{"agent":{"pane_id":"2-3"}}}`
	var resp struct {
		Result struct {
			Agent struct{ PaneID string `json:"pane_id"` } `json:"agent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.Agent.PaneID != "2-3" {
		t.Errorf("pane_id = %q, want 2-3", resp.Result.Agent.PaneID)
	}
}

func TestWaitOutputParsesMatchedLine(t *testing.T) {
	raw := `{"result":{"matched_line":"RALPH_DONE:0"}}`
	var resp struct {
		Result struct{ MatchedLine string `json:"matched_line"` } `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.MatchedLine != "RALPH_DONE:0" {
		t.Errorf("matched_line = %q, want RALPH_DONE:0", resp.Result.MatchedLine)
	}
}
