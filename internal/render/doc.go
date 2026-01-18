// Package render provides human-readable and JSON output for AF framework types.
//
// # Architectural Note
//
// This package is intentionally comprehensive because output presentation requires
// knowledge of all domain types (node, state, schema, jobs). The size and number
// of imports are necessary: splitting would either create reverse-dependency
// coupling or require complex visitor patterns that would obscure the code.
//
// The package is structured with a view model pattern to minimize coupling:
//
//   - viewmodels.go: Defines view model structs (plain data, no domain imports)
//   - adapters.go: Converts domain types to view models (the ONLY file with domain imports)
//   - render_views.go: Renders view models to strings (no domain imports)
//   - Other files: Specialized rendering (tree, status, jobs, etc.)
//
// This architecture allows the rendering logic to be tested independently of
// domain types and keeps the coupling to domain packages isolated to a single file.
package render
