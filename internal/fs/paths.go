// Package fs provides filesystem operations for the AF proof framework.
package fs

import "path/filepath"

// PathResolver provides standardized path resolution for proof directories.
type PathResolver struct {
	basePath string
}

// NewPathResolver creates a PathResolver for the given base directory.
func NewPathResolver(basePath string) *PathResolver {
	return &PathResolver{basePath: basePath}
}

// Base returns the base proof directory path.
func (p *PathResolver) Base() string { return p.basePath }

// Ledger returns the path to the ledger directory.
func (p *PathResolver) Ledger() string { return filepath.Join(p.basePath, "ledger") }

// Nodes returns the path to the nodes directory.
func (p *PathResolver) Nodes() string { return filepath.Join(p.basePath, "nodes") }

// Defs returns the path to the definitions directory.
func (p *PathResolver) Defs() string { return filepath.Join(p.basePath, "defs") }

// Assumptions returns the path to the assumptions directory.
func (p *PathResolver) Assumptions() string { return filepath.Join(p.basePath, "assumptions") }

// Externals returns the path to the externals directory.
func (p *PathResolver) Externals() string { return filepath.Join(p.basePath, "externals") }

// Lemmas returns the path to the lemmas directory.
func (p *PathResolver) Lemmas() string { return filepath.Join(p.basePath, "lemmas") }

// Locks returns the path to the locks directory.
func (p *PathResolver) Locks() string { return filepath.Join(p.basePath, "locks") }

// Meta returns the path to the meta.json file.
func (p *PathResolver) Meta() string { return filepath.Join(p.basePath, "meta.json") }
