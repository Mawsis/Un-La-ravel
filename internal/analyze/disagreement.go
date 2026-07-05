// Package analyze correlates the separately-extracted Nodes of the Project
// Model (ADR 0001) to surface findings that no single extractor could see on
// its own. It lives apart from internal/extract/{schema,model} because a
// correlation spans both the Schema (tables/columns) and the Models
// (Eloquent classes and their relationships); neither extractor owns the
// other's data.
//
// This slice produces one kind of finding: the Disagreement (see vault
// CONTEXT.md) — an Eloquent Relationship that references a table or
// foreign-key column the extracted Schema does not contain.
//
// The correlation is deliberately light and in-memory, by name only: it does
// NOT build the project-wide symbol table of ADR 0006. It joins Models to
// Tables on table name and, for explicit foreign keys, Tables to Columns on
// column name. That is sufficient for this slice and keeps the check a pure,
// order-deterministic function suitable for golden tests.
package analyze

import (
	"fmt"

	"github.com/Mawsis/Un-La-ravel/internal/model"
	"github.com/Mawsis/Un-La-ravel/internal/tablematch"
)

// FindDisagreements correlates the extracted Models against the extracted
// Schema tables and returns the Disagreements it finds — relationships whose
// target table or explicit foreign-key column is absent from the Schema.
//
// The result is deterministic and stable for golden tests: Disagreements are
// emitted in Model-discovery order, then relationship-declaration order within
// each Model. No sorting is applied because the inputs already carry a
// meaningful order (Models in first-discovery order, Relationships in
// source-declaration order), and preserving it keeps findings traceable back
// to the source.
//
// It is a pure function: it reads its arguments, allocates fresh lookup maps,
// and mutates nothing the caller passed in.
//
// Two kinds of finding are produced:
//
//   - DisagreementMissingTable — the relationship's target table is absent
//     from the Schema. The target table is the target Model's Table when that
//     Model was extracted; when the target Model is unknown (not among the
//     extracted Models) its table cannot be known, so that case is reported as
//     a missing_table Disagreement whose Reason notes the target is unknown.
//
//   - DisagreementMissingFKColumn — a belongsTo relationship carries an
//     explicit ForeignKey column that is absent from the current Model's own
//     table. belongsTo stores its foreign key locally, so the column is checked
//     on the source Model's table, not the target's.
//
// False-positive control (a deliberate testing decision for this slice — see
// the PRD): the check NEVER synthesizes a conventional key to test. Only an
// explicit ForeignKey supplied by the source code is checked, and only genuine
// absences are flagged. Implicit/conventional foreign keys are not invented and
// not flagged, even when they would in fact be missing, because guessing the
// convention is too noisy for this slice. Likewise, a missing-FK-column check
// is skipped entirely when the current Model's table is itself absent from the
// Schema — the missing table is already reported, and probing a table that does
// not exist for a column would be a redundant, misleading second finding.
func FindDisagreements(models []model.Model, tables []model.Table) []model.Disagreement {
	tableColumns := indexTables(tables)
	modelTables := indexModelTables(models)
	tableNames := listTableNames(tables)

	var disagreements []model.Disagreement

	for _, m := range models {
		for _, rel := range m.Relationships {
			if d, ok := checkTarget(m, rel, modelTables, tableColumns, tableNames); ok {
				disagreements = append(disagreements, d)
			}
			if d, ok := checkForeignKey(m, rel, tableColumns); ok {
				disagreements = append(disagreements, d)
			}
		}
	}

	return disagreements
}

// indexTables builds a lookup from table name to its set of column names. The
// presence of a table key is itself the table-existence test; the inner set is
// the column-existence test.
func indexTables(tables []model.Table) map[string]map[string]struct{} {
	index := make(map[string]map[string]struct{}, len(tables))
	for _, t := range tables {
		columns := make(map[string]struct{}, len(t.Columns))
		for _, c := range t.Columns {
			columns[c.Name] = struct{}{}
		}
		index[t.Name] = columns
	}
	return index
}

// indexModelTables builds a lookup from Model name to the table that Model maps
// to, so a relationship's target Model name can be resolved to a table name. A
// target name absent from this map is an unknown Model whose table cannot be
// determined.
func indexModelTables(models []model.Model) map[string]string {
	index := make(map[string]string, len(models))
	for _, m := range models {
		index[m.Name] = m.Table
	}
	return index
}

// listTableNames lists the schema tables' names in discovery order, the form
// tablematch.Reconcile consumes.
func listTableNames(tables []model.Table) []string {
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name)
	}
	return names
}

// checkTarget verifies that a relationship's target resolves to a table present
// in the Schema. It returns a DisagreementMissingTable finding (and true) when
// the target Model is unknown, or when the target Model's table is absent from
// the Schema; otherwise it returns the zero Disagreement and false.
//
// When the missing table has an unambiguous singular/plural sibling in the
// Schema (issue #36: the inferred "waiter_callses" for a real "waiter_calls"),
// the Reason becomes actionable: it names the real table and suggests the
// `protected $table` fix on the target Model.
func checkTarget(
	m model.Model,
	rel model.Relationship,
	modelTables map[string]string,
	tableColumns map[string]map[string]struct{},
	tableNames []string,
) (model.Disagreement, bool) {
	targetTable, known := modelTables[rel.Target]
	if !known {
		return model.Disagreement{
			Model:        m.Name,
			Relationship: rel.Method,
			Reason: fmt.Sprintf(
				"target model %q is not among the extracted models, so its table cannot be resolved",
				rel.Target,
			),
			Kind: model.DisagreementMissingTable,
		}, true
	}

	if _, ok := tableColumns[targetTable]; !ok {
		reason := fmt.Sprintf(
			"target table %q for model %q not found in schema",
			targetTable, rel.Target,
		)
		if real, matched := tablematch.Reconcile(targetTable, tableNames); matched {
			reason += fmt.Sprintf(
				" — did you mean %q? add protected $table = '%s' to %s",
				real, real, rel.Target,
			)
		}
		return model.Disagreement{
			Model:        m.Name,
			Relationship: rel.Method,
			Reason:       reason,
			Kind:         model.DisagreementMissingTable,
		}, true
	}

	return model.Disagreement{}, false
}

// checkForeignKey verifies that a belongsTo relationship's explicit foreign-key
// column exists on the current Model's own table, because belongsTo stores its
// foreign key locally. It returns a DisagreementMissingFKColumn finding (and
// true) only when the source supplied an explicit ForeignKey AND the current
// Model's table is present in the Schema AND that table lacks the column.
//
// No conventional foreign key is synthesized: an empty ForeignKey is skipped
// rather than guessed. When the current Model's table is itself absent from the
// Schema the check is skipped, leaving that to checkTarget's missing-table
// reporting and avoiding a redundant finding against a non-existent table.
func checkForeignKey(
	m model.Model,
	rel model.Relationship,
	tableColumns map[string]map[string]struct{},
) (model.Disagreement, bool) {
	if rel.Kind != relationshipBelongsTo {
		return model.Disagreement{}, false
	}
	if rel.ForeignKey == "" {
		return model.Disagreement{}, false
	}

	columns, tableKnown := tableColumns[m.Table]
	if !tableKnown {
		return model.Disagreement{}, false
	}

	if _, ok := columns[rel.ForeignKey]; ok {
		return model.Disagreement{}, false
	}

	return model.Disagreement{
		Model:        m.Name,
		Relationship: rel.Method,
		Reason: fmt.Sprintf(
			"foreign key column %q not found on table %q",
			rel.ForeignKey, m.Table,
		),
		Kind: model.DisagreementMissingFKColumn,
	}, true
}

// relationshipBelongsTo is the Eloquent relationship kind whose foreign key is
// stored on the declaring Model's own table. It is the only kind whose explicit
// ForeignKey is checked against the current Model's table in this slice.
const relationshipBelongsTo = "belongsTo"
