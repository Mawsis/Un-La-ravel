package middleware

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// bootstrapPath is bootstrap/app.php relative to a project root — the Laravel
// 11+ home of the middleware configuration that a ≤10 app declared as four
// properties on app/Http/Kernel.php (issue #67). The framework replaced those
// literals with imperative calls on a Middleware configurator inside the
// `Application::configure()->withMiddleware(closure)->create()` chain, so the
// same alias→class map, group memberships, global stack, and priority ordering
// are read from that closure's body instead.
var bootstrapPath = filepath.Join("bootstrap", "app.php")

// The Middleware configurator methods this reader understands. Laravel exposes
// many more (and several accept shapes no static read can resolve); these are
// the array/class-const-literal forms an application actually writes, and the
// only ones we resolve — anything else is left alone rather than guessed
// (precision over coverage, ADR 0002).
const (
	// aliasMethod declares the alias→class map: ->alias(['auth' => X::class]).
	aliasMethod = "alias"
	// groupMethod defines a group wholesale: ->group('web', [X::class, ...]).
	groupMethod = "group"
	// appendToGroupMethod / prependToGroupMethod add to an existing group:
	// ->appendToGroup('api', [X::class]). Both contribute the same membership —
	// position within a group is not modelled on the node.
	appendToGroupMethod  = "appendToGroup"
	prependToGroupMethod = "prependToGroup"
	// webMethod / apiMethod are the shorthands for the two groups Laravel ships,
	// and are what its documentation leads with: ->web(append: [X::class]) is
	// ->appendToGroup('web', [X::class]). The group name is the method name.
	webMethod = "web"
	apiMethod = "api"
	// appendMethod / prependMethod add to the global stack: ->append(X::class)
	// or ->append([X::class, ...]). Both mean "runs on every request".
	appendMethod  = "append"
	prependMethod = "prepend"
	// priorityMethod sets the execution ordering: ->priority([X::class, ...]).
	priorityMethod = "priority"
)

// ReadDeclared reads a project's declared middleware from whichever file
// declares it, and returns nil when the project declares none.
//
// Laravel ≤10 declares it as properties on app/Http/Kernel.php (ReadKernel);
// Laravel 11+ moved it into the ->withMiddleware() closure of bootstrap/app.php
// (ReadBootstrap). The Kernel is tried first and WINS OUTRIGHT when present: a
// project mid-migration can have both files on disk, and one that still ships a
// Kernel class is still booting through it, so the Kernel holds the live
// mapping. See ADR 0012 §1a for why matching the running app beats preferring
// the newer file.
//
// This is the entry point every caller should use; reaching for ReadKernel or
// ReadBootstrap directly means re-deciding the precedence rule, and getting it
// wrong is silent — the analysis simply reports a tier that isn't there.
func ReadDeclared(projectPath string) (*Kernel, error) {
	kernel, err := ReadKernel(projectPath)
	if err != nil {
		return nil, err
	}
	if kernel != nil {
		return kernel, nil
	}
	return ReadBootstrap(projectPath)
}

// ReadBootstrap reads bootstrap/app.php under projectPath and returns the
// middleware declared in its `->withMiddleware(...)` closure in the same Kernel
// shape the ≤10 reader produces, so Extract consumes both sources identically.
//
// It returns (nil, nil) when the file is absent (a Laravel ≤10 project, whose
// middleware comes from app/Http/Kernel.php) and also when the file exists but
// has no readable `->withMiddleware(closure)` link — a bare Laravel 11 skeleton
// declares nothing, which is indistinguishable in effect from having no file:
// either way there is no app tier and the built-in backstop still covers
// framework aliases. A read or catastrophic parse failure of a file that IS
// present is returned as a wrapped error, mirroring ReadKernel.
//
// Within the closure only the statically readable call forms are resolved (the
// method constants above): a call whose group name is not a string literal, or
// whose middleware argument is not a class-const or a list of them, contributes
// nothing rather than being guessed (ADR 0002).
//
// The configurator methods this reader deliberately does NOT model, and what
// that costs:
//
//   - ->removeFromGroup() / ->replaceInGroup(): SUBTRACTIVE. Ignoring them can
//     leave a class listed in a group it was later removed or replaced out of,
//     so a group membership is "declared at some point" rather than "final".
//     This is the one omission that can make a node's Groups over-report; it is
//     accepted because both calls are rare and reading them correctly means
//     ordering every mutation, not just collecting literals.
//   - ->redirectGuestsTo(), ->trustProxies(), ->encryptCookies(),
//     ->validateCsrfTokens(), ->throttleApi(), ->statefulApi(),
//     ->convertEmptyStringsToNull() and the rest of the tuning surface: these
//     configure the BEHAVIOUR of middleware the node set already knows about
//     rather than declaring an alias, a group membership, or a global, so they
//     contribute nothing to this model by construction — not a gap.
//
// Anything else — a fluent mutation chain, a call whose arguments are computed
// at runtime — is likewise skipped rather than guessed.
func ReadBootstrap(projectPath string) (*Kernel, error) {
	path := filepath.Join(projectPath, bootstrapPath)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, nil
	}

	res, err := phpast.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bootstrap application file %s: %w", path, err)
	}

	stmts := phpast.WithMiddlewareStmts(res.Root)
	if stmts == nil {
		return nil, nil
	}

	k := newKernel()
	for _, stmt := range stmts {
		_, method, args, ok := phpast.MethodCallParts(phpast.ExpressionStmt(stmt))
		if !ok {
			continue
		}
		k.applyConfiguratorCall(phpast.CallName(method), args)
	}
	if k.isEmpty() {
		// The closure exists but nothing in it was statically readable. That is
		// the same state as having no closure at all — no declared tier — so
		// report it identically rather than handing back an empty Kernel that
		// every caller would have to special-case.
		return nil, nil
	}
	return k, nil
}

// applyConfiguratorCall folds one `$middleware->method(...)` call from the
// withMiddleware closure into the Kernel's read-time state. An unmodelled method
// name, or a modelled one whose arguments are not literal, is a no-op.
func (k *Kernel) applyConfiguratorCall(method string, args []phpast.Vertex) {
	switch method {
	case aliasMethod:
		// ->alias(['auth' => Authenticate::class, ...]) — the alias→class map,
		// read in declaration order exactly as $middlewareAliases is.
		for _, pair := range phpast.ArrayClassConstPairs(phpast.ArgExpr(args, 0)) {
			k.addAlias(pair.Key, pair.Class)
		}

	case groupMethod, appendToGroupMethod, prependToGroupMethod:
		// ->group('web', [...]) / ->appendToGroup('api', [...]) — the group name
		// is the first argument and must be a string literal; the members are the
		// second. Membership is order-of-appearance across calls, matching how
		// $middlewareGroups' declaration order drives the ≤10 path.
		name := phpast.NthStringArg(args, 0)
		if name == "" {
			return
		}
		k.addGroupMembers(name, phpast.ArgExpr(args, 1))

	case webMethod, apiMethod:
		// ->web([...]) / ->api(append: [...], prepend: [...]) — the shorthands for
		// the two groups Laravel ships. The group name IS the method name, and the
		// members arrive either positionally or under the append:/prepend: labels;
		// both land in the same group, since position within a group is not
		// modelled on the node.
		for _, list := range phpast.NamedArgExprs(args, "append", "prepend") {
			k.addGroupMembers(method, list)
		}

	case appendMethod, prependMethod:
		// ->append(X::class) or ->append([X::class, ...]) — the global stack.
		// Laravel accepts both the bare and the list form, so read the bare
		// class-const argument first and fall back to the list.
		if class := phpast.NthArgClassConst(args, 0); class != "" {
			k.addGlobal(class)
			return
		}
		for _, class := range phpast.ArrayClassConstItems(phpast.ArgExpr(args, 0)) {
			k.addGlobal(class)
		}

	case priorityMethod:
		// ->priority([...]) — the 1-based execution ordering, positioned by index
		// in the list exactly as $middlewarePriority is on the ≤10 path.
		k.addPriorityList(phpast.ArrayClassConstItems(phpast.ArgExpr(args, 0)))
	}
}
