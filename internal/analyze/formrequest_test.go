package analyze

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Mawsis/Un-La-ravel/internal/extract/controller"
	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/symbol"
)

// resolvedRoute is a terse constructor for a route that has already been through
// ResolveRoutes: it carries the resolved controller FQN and the action name that
// LinkFormRequests keys ActionParams by.
func resolvedRoute(method, uri, ctrlFQN, action string) model.Route {
	return model.Route{Method: method, URI: uri, FQN: ctrlFQN, Action: action}
}

// writeControllerFile writes a controller file with the given contents into a
// temp dir, adds it to the symbol table (so its `use` imports back parameter-type
// resolution), and returns its path — the fromFile context LinkFormRequests
// resolves a short request-type name against.
func writeControllerFile(t *testing.T, tbl *symbol.Table, php string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "controller.php")
	if err := os.WriteFile(path, []byte(php), 0o600); err != nil {
		t.Fatalf("write controller file: %v", err)
	}
	if err := tbl.AddFile(path); err != nil {
		t.Fatalf("add controller file: %v", err)
	}
	return path
}

// TestLinkFormRequests drives the Route→FormRequest link through its contract
// cases: an imported request type resolves and is linked; an unimported short
// name falls back to Laravel's default request namespace; a fully-qualified
// parameter type is linked verbatim; an action whose parameter is not a
// FormRequest is left unlinked; a route with no resolved controller FQN is passed
// through; the first FormRequest parameter wins when several are typed; and an
// already-set FormRequest is never overwritten.
func TestLinkFormRequests(t *testing.T) {
	const postCtrl = `App\Http\Controllers\PostController`
	const storeReq = `App\Http\Requests\StorePostRequest`

	formRequests := []model.FormRequest{
		model.NewFormRequest("StorePostRequest", storeReq),
	}

	tests := []struct {
		name          string
		controllerPHP string
		routes        []model.Route
		actionParams  controller.ActionParams
		want          []model.Route
	}{
		{
			name: "imported request type resolves and is linked",
			controllerPHP: `<?php
namespace App\Http\Controllers;
use App\Http\Requests\StorePostRequest;
class PostController {}
`,
			routes:       []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
			actionParams: controller.ActionParams{postCtrl: {"store": {"StorePostRequest"}}},
			want: []model.Route{{
				Method: "POST", URI: "/posts", FQN: postCtrl, Action: "store",
				FormRequest: storeReq,
			}},
		},
		{
			name: "unimported short name falls back to the default request namespace",
			controllerPHP: `<?php
namespace App\Http\Controllers;
class PostController {}
`,
			routes:       []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
			actionParams: controller.ActionParams{postCtrl: {"store": {"StorePostRequest"}}},
			want: []model.Route{{
				Method: "POST", URI: "/posts", FQN: postCtrl, Action: "store",
				FormRequest: storeReq,
			}},
		},
		{
			name: "fully-qualified parameter type is linked verbatim",
			controllerPHP: `<?php
namespace App\Http\Controllers;
class PostController {}
`,
			routes: []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
			actionParams: controller.ActionParams{
				postCtrl: {"store": {`App\Http\Requests\StorePostRequest`}},
			},
			want: []model.Route{{
				Method: "POST", URI: "/posts", FQN: postCtrl, Action: "store",
				FormRequest: storeReq,
			}},
		},
		{
			name: "non-FormRequest parameter is left unlinked",
			controllerPHP: `<?php
namespace App\Http\Controllers;
use App\Services\PostService;
class PostController {}
`,
			routes:       []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
			actionParams: controller.ActionParams{postCtrl: {"store": {"PostService"}}},
			want:         []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
		},
		{
			name: "action with no typed parameter is left unlinked",
			controllerPHP: `<?php
namespace App\Http\Controllers;
class PostController {}
`,
			routes:       []model.Route{resolvedRoute("GET", "/posts", postCtrl, "index")},
			actionParams: controller.ActionParams{}, // no entry for this action
			want:         []model.Route{resolvedRoute("GET", "/posts", postCtrl, "index")},
		},
		{
			name: "route with no resolved controller FQN is passed through",
			controllerPHP: `<?php
namespace App\Http\Controllers;
class PostController {}
`,
			routes:       []model.Route{{Method: "POST", URI: "/posts", Action: "store"}},
			actionParams: controller.ActionParams{postCtrl: {"store": {"StorePostRequest"}}},
			want:         []model.Route{{Method: "POST", URI: "/posts", Action: "store"}},
		},
		{
			name: "first FormRequest parameter wins over a later non-request one",
			controllerPHP: `<?php
namespace App\Http\Controllers;
use App\Http\Requests\StorePostRequest;
class PostController {}
`,
			routes: []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")},
			actionParams: controller.ActionParams{
				postCtrl: {"store": {"StorePostRequest", "PostService"}},
			},
			want: []model.Route{{
				Method: "POST", URI: "/posts", FQN: postCtrl, Action: "store",
				FormRequest: storeReq,
			}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tbl := symbol.New()
			path := writeControllerFile(t, tbl, tt.controllerPHP)
			controllerFiles := map[string]string{postCtrl: path}

			got := LinkFormRequests(tt.routes, formRequests, tt.actionParams, tbl, controllerFiles)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("routes = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

// TestLinkFormRequestsDoesNotOverwriteExistingLink verifies the link is additive:
// a route that already carries a FormRequest keeps it, even when its action's
// parameter would resolve to a different one.
func TestLinkFormRequestsDoesNotOverwriteExistingLink(t *testing.T) {
	const postCtrl = `App\Http\Controllers\PostController`
	const alreadyLinked = `App\Http\Requests\ExistingRequest`

	tbl := symbol.New()
	path := writeControllerFile(t, tbl, `<?php
namespace App\Http\Controllers;
use App\Http\Requests\StorePostRequest;
class PostController {}
`)

	routes := []model.Route{{
		Method: "POST", URI: "/posts", FQN: postCtrl, Action: "store",
		FormRequest: alreadyLinked,
	}}
	formRequests := []model.FormRequest{
		model.NewFormRequest("StorePostRequest", `App\Http\Requests\StorePostRequest`),
	}
	actionParams := controller.ActionParams{postCtrl: {"store": {"StorePostRequest"}}}

	got := LinkFormRequests(routes, formRequests, actionParams, tbl, map[string]string{postCtrl: path})
	if got[0].FormRequest != alreadyLinked {
		t.Errorf("FormRequest = %q, want the pre-existing %q (link must not overwrite)", got[0].FormRequest, alreadyLinked)
	}
}

// TestLinkFormRequestsDoesNotMutateInputs verifies purity: the returned routes
// are a fresh slice and the caller's routes are untouched.
func TestLinkFormRequestsDoesNotMutateInputs(t *testing.T) {
	const postCtrl = `App\Http\Controllers\PostController`

	tbl := symbol.New()
	path := writeControllerFile(t, tbl, `<?php
namespace App\Http\Controllers;
use App\Http\Requests\StorePostRequest;
class PostController {}
`)

	routes := []model.Route{resolvedRoute("POST", "/posts", postCtrl, "store")}
	formRequests := []model.FormRequest{
		model.NewFormRequest("StorePostRequest", `App\Http\Requests\StorePostRequest`),
	}
	actionParams := controller.ActionParams{postCtrl: {"store": {"StorePostRequest"}}}

	wantRoutes := append([]model.Route(nil), routes...)

	got := LinkFormRequests(routes, formRequests, actionParams, tbl, map[string]string{postCtrl: path})

	if !reflect.DeepEqual(routes, wantRoutes) {
		t.Errorf("LinkFormRequests mutated the routes input: got %#v, want %#v", routes, wantRoutes)
	}
	if len(got) > 0 && &got[0] == &routes[0] {
		t.Error("LinkFormRequests returned the same backing array as the input routes")
	}
}
