package findings

import (
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/model"
)

// authRoute is a tiny helper: a Route carrying just the fields the auth findings
// read (method, URI, middleware). Keeps the table tests below focused on the
// method→severity split without constructing full routes.
func authRoute(method, uri string, mw ...string) model.Route {
	return model.Route{Method: method, URI: uri, Middleware: mw}
}

// TestItems_UnauthenticatedWriteAndRead is the finding-computation tracer bullet:
// an unauthenticated WRITE route (POST) flattens to a FindingUnauthenticatedWrite
// item and an unauthenticated READ route (GET) to a FindingUnauthenticatedRead
// item, each carrying the route's method and URI so the baseline can fingerprint
// it. Authenticated and unknown routes produce no auth item at all — a finding is
// a problem, and only routes reachable without auth are the problem.
func TestItems_UnauthenticatedWriteAndRead(t *testing.T) {
	pm := model.New("t", "10.x")
	pm.AddRoute(authRoute("GET", "/posts"))            // unauth read  → warn item
	pm.AddRoute(authRoute("POST", "/webhooks"))        // unauth write → blocker item
	pm.AddRoute(authRoute("POST", "/posts", "auth"))   // authenticated → no item
	pm.AddRoute(authRoute("DELETE", "/x", "verified")) // unknown       → no item

	items := Items(pm)

	var writes, reads []Item
	for _, it := range items {
		switch it.Kind {
		case model.FindingUnauthenticatedWrite:
			writes = append(writes, it)
		case model.FindingUnauthenticatedRead:
			reads = append(reads, it)
		}
	}

	if len(writes) != 1 || writes[0].Method != "POST" || writes[0].URI != "/webhooks" {
		t.Errorf("want one unauthenticated-write item POST /webhooks, got %+v", writes)
	}
	if len(reads) != 1 || reads[0].Method != "GET" || reads[0].URI != "/posts" {
		t.Errorf("want one unauthenticated-read item GET /posts, got %+v", reads)
	}
}

// TestItems_WriteMethodsAreBlockers pins the exact verb→category split: every
// mutating verb (POST/PUT/PATCH/DELETE) with no auth is a write finding; GET (and
// HEAD/OPTIONS, which don't mutate) are reads. The split is what makes an
// unauthenticated write a blocker and an unauthenticated read a warning.
func TestItems_WriteMethodsAreBlockers(t *testing.T) {
	writeVerbs := []string{"POST", "PUT", "PATCH", "DELETE"}
	for _, verb := range writeVerbs {
		pm := model.New("t", "10.x")
		pm.AddRoute(authRoute(verb, "/r"))
		items := Items(pm)
		if len(items) != 1 || items[0].Kind != model.FindingUnauthenticatedWrite {
			t.Errorf("%s with no auth: want one FindingUnauthenticatedWrite, got %+v", verb, items)
		}
	}
	pm := model.New("t", "10.x")
	pm.AddRoute(authRoute("GET", "/r"))
	items := Items(pm)
	if len(items) != 1 || items[0].Kind != model.FindingUnauthenticatedRead {
		t.Errorf("GET with no auth: want one FindingUnauthenticatedRead, got %+v", items)
	}
}

// TestVerdict_AuthCategoriesCountAndLabel confirms the category rollup Verdict
// emits: two unauthenticated writes and one unauthenticated read produce a
// blocker-severity "2 unauthenticated write routes" finding and a warn-severity
// "1 unauthenticated read route" finding, pluralized to match the count.
func TestVerdict_AuthCategoriesCountAndLabel(t *testing.T) {
	pm := model.New("t", "10.x")
	pm.AddRoute(authRoute("POST", "/a"))
	pm.AddRoute(authRoute("DELETE", "/b"))
	pm.AddRoute(authRoute("GET", "/c"))
	pm.AddRoute(authRoute("GET", "/ok", "auth")) // authenticated, no finding

	byKind := map[string]model.Finding{}
	for _, f := range Verdict(pm) {
		byKind[f.Kind] = f
	}

	w, ok := byKind[model.FindingUnauthenticatedWrite]
	if !ok || w.Count != 2 || w.Severity != model.SeverityBlocker || w.Label != "2 unauthenticated write routes" {
		t.Errorf("unauthenticated-write finding = %+v (present=%v), want count 2, blocker, plural label", w, ok)
	}
	r, ok := byKind[model.FindingUnauthenticatedRead]
	if !ok || r.Count != 1 || r.Severity != model.SeverityWarn || r.Label != "1 unauthenticated read route" {
		t.Errorf("unauthenticated-read finding = %+v (present=%v), want count 1, warn, singular label", r, ok)
	}
	// Auth findings link to the auth view, not the generic findings view, so the
	// itemized verdict entry is clickable to where the detail actually lives.
	if w.View != "auth" || r.View != "auth" {
		t.Errorf("auth findings View = write %q read %q, want both %q", w.View, r.View, "auth")
	}
}
