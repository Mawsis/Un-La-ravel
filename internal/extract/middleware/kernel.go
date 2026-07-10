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

// Kernel is the declared middleware data read from a Laravel ≤10
// app/Http/Kernel.php: the alias→class map (in declaration order), the group
// memberships, the global stack, and the priority ordering. It is the "app"
// tier's source — the extractor turns it into origin-"app" nodes with resolved
// classes, groups, global flags, and priorities, ahead of the built-in backstop
// (ADR 0012).
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

	k := &Kernel{
		aliasClass:    make(map[string]string),
		classGroups:   make(map[string][]string),
		globalClasses: make(map[string]struct{}),
		classPriority: make(map[string]int),
	}

	// Alias → class map. Laravel 9+ names it $middlewareAliases; Laravel ≤8 named
	// the same map $routeMiddleware. Prefer the modern name, fall back to legacy.
	aliasArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewareAliases")
	if aliasArr == nil {
		aliasArr = phpast.ClassPropertyArray(root, kernelClassName, "routeMiddleware")
	}
	for _, pair := range phpast.ArrayClassConstPairs(aliasArr) {
		if _, dup := k.aliasClass[pair.Key]; dup {
			continue // first declaration of an alias wins; keep emit order stable.
		}
		k.aliasOrder = append(k.aliasOrder, pair.Key)
		k.aliasClass[pair.Key] = pair.Class
	}

	// Group memberships: class → groups it belongs to, in group-declaration order.
	groupsArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewareGroups")
	for _, group := range phpast.ArrayClassConstGroups(groupsArr) {
		for _, class := range group.Classes {
			k.classGroups[class] = append(k.classGroups[class], group.Key)
		}
	}

	// Global stack: the classes that run on every request.
	globalArr := phpast.ClassPropertyArray(root, kernelClassName, "middleware")
	for _, class := range phpast.ArrayClassConstItems(globalArr) {
		k.globalClasses[class] = struct{}{}
	}

	// Priority ordering: 1-based position so the zero value means "not listed".
	priorityArr := phpast.ClassPropertyArray(root, kernelClassName, "middlewarePriority")
	for i, class := range phpast.ArrayClassConstItems(priorityArr) {
		if _, dup := k.classPriority[class]; dup {
			continue // first (highest) position wins for a repeated class.
		}
		k.classPriority[class] = i + 1
	}

	return k, nil
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
