// Package web_test — reskin_test.go covers issue #38: the working-view reskin
// (calm register). Like restyle_test.go it asserts on the served asset bytes —
// the design system lives entirely in static assets, so the shipped CSS/HTML
// text IS the public interface — and it mechanizes the PRD's grep-able
// anti-reference checklist so slop cannot regress:
//
//   - no colored side-stripe accents (border-left/right wider than 1px);
//     severity reads as a leading status dot + a background tint instead;
//   - uppercase is reserved for true section headings (the nav group labels);
//   - the inventory stat strip is quiet by size, not by a dimmed-numbers
//     CSS override;
//   - the hero value prop carries no em dash;
//   - ER edges style resolved (cyan) vs unresolved (red) via the thread-role
//     tokens.
package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// sideStripeRe matches a border-left/right declaration whose width is 2px or
// more — the colored side-stripe accent the PRD bans outright. A 1px border is
// a hairline, not a stripe; anything wider on one side is doing severity work
// the status dot + tint now own. border-left-color is included because a
// color-only override exists solely to repaint a stripe declared elsewhere.
var sideStripeRe = regexp.MustCompile(`border-(left|right)(-color)?:\s*(([2-9]|\d{2,})px)?[^;}]*`)

// TestReskin_NoSideStripes is the tracer bullet for issue #38: no shell
// stylesheet may declare a side-stripe. The finding cards, model risk cards,
// and active nav all carried one; after the reskin the only legal
// border-left/right is a 1px hairline (the sidebar's border-right).
func TestReskin_NoSideStripes(t *testing.T) {
	for _, sheet := range shellStylesheets {
		css := fetchAsset(t, sheet)
		for _, match := range sideStripeRe.FindAllString(css, -1) {
			if isHairline(match) {
				continue
			}
			t.Errorf("%s declares side-stripe %q — severity reads as status dot + tint, never a colored side border", sheet, strings.TrimSpace(match))
		}
	}
}

// isHairline reports whether a border-left/right declaration is the allowed
// 1px structural hairline rather than a stripe.
func isHairline(decl string) bool {
	return strings.Contains(decl, "1px") && !regexp.MustCompile(`\d{2,}px`).MatchString(decl)
}
