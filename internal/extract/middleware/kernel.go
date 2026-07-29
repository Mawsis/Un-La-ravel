package middleware

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mawsis/Un-La-ravel/internal/phpast"
)

// kernelPath is app/Http/Kernel.php relative to a project root — the Laravel ≤10
// HTTP Kernel that declares the alias→class map, middleware groups, the global
// stack, and the priority ordering (issue #66). Laravel 11+ moved this into
// bootstrap/app.php's ->withMiddleware() closure, read by a later slice (#67).
var kernelPath = filepath.Join("app", "Http", "Kernel.php")

// kernelClassName is the conventional class name of the HTTP Kernel. A Laravel
// app's Kernel is `class Kernel extends ...HttpKernel`; the reader locates the
// properties on this named class so an unrelated class in the same file is not
// mistaken for it.
const kernelClassName = "Kernel"

// Kernel is the declared middleware data of a project, whichever file declares
// it: the alias→class map (in declaration order), the group memberships, the
// global stack, and the priority ordering. It is the "app" tier's source — the
// extractor turns it into origin-"app" nodes with resolved classes, groups,
// global flags, and priorities, ahead of the built-in backstop (ADR 0012).
//
// The type is named for its original Laravel ≤10 source, app/Http/Kernel.php
// (ReadKernel, issue #66), but is deliberately source-agnostic: the Laravel 11+
// bootstrap/app.php reader (ReadBootstrap, bootstrap.go, issue #67) populates
// the same fields through the same mutators, so Extract consumes both layouts
// through one shape and the two paths cannot drift.
//
// Order is preserved ONLY where it is emitted: aliasOrder drives the app tier's
// emit order (the determinism invariant). The lookup maps below are read-time
// helper state that never reaches serialized JSON, so — like the model
// extractor's internal maps — they are free to be Go maps.
type Kernel struct {
	// aliasOrder is the alias names in the order $middlewareAliases (or the legacy
	// $routeMiddleware) declared them — the app tier's emit order.
	aliasOrder []string
	// aliasClass maps an alias to its resolved class FQN (no leading backslash).
	aliasClass map[string]string
	// classGroups maps a middleware class FQN to the groups it belongs to, in
	// $middlewareGroups declaration order across groups.
	classGroups map[string][]string
	// globalClasses is the set of class FQNs in the global $middleware stack.
	globalClasses map[string]struct{}
	// classPriority maps a class FQN to its 1-based position in
	// $middlewarePriority (0, the zero value, means "not listed").
	classPriority map[string]int
}

// ReadKernel reads app/Http/Kernel.php under projectPath and returns the declared
// middleware data, or (nil, nil) when the file is absent — a project with no ≤10
// Kernel (a Laravel 11+ app, or one mid-migration) is not an error; it simply
// contributes no app tier and the built-in backstop still covers framework
// aliases. A read or catastrophic parse failure of a Kernel that IS present is
// returned as a wrapped error, mirroring the other extractors: a Kernel we fail
// to read would silently drop every declared alias's resolution.
//
// The four properties are read through phpast helpers (parser isolation, ADR
// 0003): $middlewareAliases (falling back to the legacy $routeMiddleware name)
// for the alias→class map, $middlewareGroups for group membership, $middleware
// for the global stack, and $middlewarePriority for the ordering.
func ReadKernel(projectPath string) (*Kernel, error) {
	path := filepath.Join(projectPath, kernelPath)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, nil
	}

	res, err := phpast.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP Kernel %s: %w", path, err)
	}
	root := res.Root

	k := newKernel()

	// Alias → class map. Laravel 9+ names it $middlewareAliases; Laravel ≤8 named
	// the same map $routeMiddleware. Prefer the modern name, fall back to legacy.
	aliasArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewareAliases")
	if aliasArr == nil {
		aliasArr = phpast.ClassPropertyArray(root, kernelClassName, "routeMiddleware")
	}
	for _, pair := range phpast.ArrayClassConstPairs(aliasArr) {
		k.addAlias(pair.Key, pair.Class)
	}

	// Group memberships: class → groups it belongs to, in group-declaration order.
	groupsArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewareGroups")
	for _, group := range phpast.ArrayClassConstGroups(groupsArr) {
		for _, class := range group.Classes {
			k.addGroupMember(group.Key, class)
		}
	}

	// Global stack: the classes that run on every request.
	globalArr := phpast.ClassPropertyArray(root, kernelClassName, "middleware")
	for _, class := range phpast.ArrayClassConstItems(globalArr) {
		k.addGlobal(class)
	}

	// Priority ordering: 1-based position so the zero value means "not listed".
	priorityArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewarePriority")
	k.addPriorityList(phpast.ArrayClassConstItems(priorityArr))

	return k, nil
}

// isEmpty reports whether nothing at all was read into the Kernel — no alias, no
// group membership, no global, no priority. A reader uses this to collapse a
// "found the declaration site but could read nothing from it" outcome to the
// same nil no-declared-tier result a missing declaration site gives.
func (k *Kernel) isEmpty() bool {
	return len(k.aliasOrder) == 0 && len(k.classGroups) == 0 &&
		len(k.globalClasses) == 0 && len(k.classPriority) == 0
}

// newKernel returns an empty Kernel with its read-time lookup maps allocated,
// ready for the addX mutators below. Both readers (ReadKernel here, ReadBootstrap
// in bootstrap.go) start from this so neither can forget a map.
func newKernel() *Kernel {
	return &Kernel{
		aliasClass:    make(map[string]string),
		classGroups:   make(map[string][]string),
		globalClasses: make(map[string]struct{}),
		classPriority: make(map[string]int),
	}
}

// addAlias records an alias→class resolution, appending the alias to the emit
// order on first sight. A repeated alias is ignored: the first declaration wins,
// which keeps the emit order stable regardless of how a source restates it.
func (k *Kernel) addAlias(alias, class string) {
	if alias == "" || class == "" {
		return
	}
	if _, dup := k.aliasClass[alias]; dup {
		return
	}
	k.aliasOrder = append(k.aliasOrder, alias)
	k.aliasClass[alias] = class
}

// addGroupMember records that class belongs to the named group, in call order.
// A class added to the same group twice keeps a single membership so a source
// that both defines and appends to a group does not double-list it.
func (k *Kernel) addGroupMember(group, class string) {
	if group == "" || class == "" {
		return
	}
	for _, existing := range k.classGroups[class] {
		if existing == group {
			return
		}
	}
	k.classGroups[class] = append(k.classGroups[class], group)
}

// addGroupMembers records every `Class::class` item of an array-literal
// expression as a member of the named group, skipping a nil or non-array
// expression. It is the bulk form of addGroupMember for the callers that hold a
// members list as an unread AST expression.
func (k *Kernel) addGroupMembers(group string, list phpast.Vertex) {
	for _, class := range phpast.ArrayClassConstItems(list) {
		k.addGroupMember(group, class)
	}
}

// addGlobal marks a class as part of the global stack (runs on every request).
func (k *Kernel) addGlobal(class string) {
	if class == "" {
		return
	}
	k.globalClasses[class] = struct{}{}
}

// addPriorityList assigns each class in a declared priority ordering its 1-based
// position in THAT list, so the zero value keeps meaning "not listed". Position
// is the index in the source list, not a running counter: a repeated class keeps
// its first (highest) position and the duplicate still consumes its slot, so the
// numbers a consumer sees line up with the positions actually written in the
// source. A second call (Laravel 11+ permits ->priority() more than once)
// restates the ordering from 1 and yields to whatever the first call positioned.
func (k *Kernel) addPriorityList(classes []string) {
	for i, class := range classes {
		if class == "" {
			continue
		}
		if _, dup := k.classPriority[class]; dup {
			continue
		}
		k.classPriority[class] = i + 1
	}
}

// AliasOrder returns the declared aliases in source order — the app tier's emit
// order. The returned slice is the Kernel's own backing slice; callers must not
// mutate it.
func (k *Kernel) AliasOrder() []string {
	if k == nil {
		return nil
	}
	return k.aliasOrder
}

// ClassFor returns the resolved class FQN for a declared alias, or "" when the
// alias is not declared.
func (k *Kernel) ClassFor(alias string) string {
	if k == nil {
		return ""
	}
	return k.aliasClass[alias]
}

// GroupsFor returns the middleware groups a class belongs to, in group-
// declaration order, or nil when the class is in no group. The returned slice is
// the Kernel's own backing slice; callers must not mutate it.
func (k *Kernel) GroupsFor(class string) []string {
	if k == nil || class == "" {
		return nil
	}
	return k.classGroups[class]
}

// IsGlobal reports whether a class is in the global $middleware stack (runs on
// every request).
func (k *Kernel) IsGlobal(class string) bool {
	if k == nil || class == "" {
		return false
	}
	_, ok := k.globalClasses[class]
	return ok
}

// PriorityFor returns a class's 1-based position in $middlewarePriority, or 0
// when the class is not listed there.
func (k *Kernel) PriorityFor(class string) int {
	if k == nil || class == "" {
		return 0
	}
	return k.classPriority[class]
}
