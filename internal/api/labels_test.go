package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/johnreginald/donewhen/internal/config"
	"github.com/johnreginald/donewhen/internal/models"
)

func TestMoveLabel(t *testing.T) {
	e := newEnv(t, config.Config{})
	owner, _ := e.user("")
	w1 := e.workspace(owner)
	w2 := e.workspace(owner)
	a := browser(e.session(owner))
	in1 := header("X-Workspace", w1.ID)

	mk := func(name, group string) models.Label {
		t.Helper()
		b := map[string]any{"name": name, "color": "#38BDF8"}
		if group != "" {
			b["group"] = group
		}
		r := e.do("POST", "/api/labels", b, a, in1)
		if r.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", name, r.Code, r.Body.String())
		}
		var l models.Label
		_ = json.Unmarshal(r.Body.Bytes(), &l)
		return l
	}
	groupID := func(name string) string {
		t.Helper()
		r := e.do("GET", "/api/label-groups", nil, a, in1)
		if r.Code != 200 {
			t.Fatalf("groups: %d %s", r.Code, r.Body.String())
		}
		var gs []models.LabelGroup
		_ = json.Unmarshal(r.Body.Bytes(), &gs)
		for _, g := range gs {
			if g.Name == name {
				return g.ID
			}
		}
		t.Fatalf("group %q not found", name)
		return ""
	}
	move := func(labelID string, group any) *httptest.ResponseRecorder {
		t.Helper()
		return e.do("PATCH", "/api/labels/"+labelID, map[string]any{"groupId": group}, a, in1)
	}

	led := mk("led", "Alpha")
	mech := mk("mech", "Beta")

	r := move(led.ID, groupID("Beta"))
	if r.Code != 200 {
		t.Fatalf("move: %d %s", r.Code, r.Body.String())
	}
	var got models.Label
	_ = json.Unmarshal(r.Body.Bytes(), &got)
	if got.ID != led.ID || got.GroupID == nil || *got.GroupID != groupID("Beta") {
		t.Fatalf("moved = %+v", got)
	}

	r = move(mech.ID, nil)
	if r.Code != 200 {
		t.Fatalf("ungroup: %d %s", r.Code, r.Body.String())
	}
	_ = json.Unmarshal(r.Body.Bytes(), &got)
	if got.ID != mech.ID || got.GroupID != nil {
		t.Fatalf("ungrouped = %+v", got)
	}

	foreign, err := e.store.CreateLabel(t.Context(), w2.ID, "theirs", "", "Gamma")
	if err != nil {
		t.Fatalf("foreign label: %v", err)
	}
	if r := move(led.ID, *foreign.GroupID); r.Code != 400 {
		t.Fatalf("foreign group: %d %s", r.Code, r.Body.String())
	}
	r = e.do("GET", "/api/labels", nil, a, in1)
	var ls []models.Label
	_ = json.Unmarshal(r.Body.Bytes(), &ls)
	for _, l := range ls {
		if l.ID == led.ID && (l.GroupID == nil || *l.GroupID != groupID("Beta")) {
			t.Fatalf("failed move changed the label: %+v", l)
		}
	}

	// A label from another workspace is not found.
	if r := move(foreign.ID, nil); r.Code != 404 {
		t.Fatalf("foreign label: %d %s", r.Code, r.Body.String())
	}

	r = e.do("PATCH", "/api/labels/"+led.ID, map[string]any{}, a, in1)
	if r.Code != 400 {
		t.Fatalf("absent groupId: %d %s", r.Code, r.Body.String())
	}

	soloA := mk("soloA", "Epsilon")
	mk("soloB", "Delta")
	if r := e.do("POST", "/api/issues", map[string]any{
		"title": "clash", "stateName": "In Progress", "labelNames": []string{"soloA", "soloB"},
	}, a, in1); r.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", r.Code, r.Body.String())
	}
	if r := move(soloA.ID, groupID("Delta")); r.Code != 400 {
		t.Fatalf("exclusive clash: %d %s", r.Code, r.Body.String())
	}
}
