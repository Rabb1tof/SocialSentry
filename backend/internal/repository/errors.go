// Package repository contains data-access interfaces and their pgx-backed implementations.
package repository

import "errors"

// ErrNotFound is returned when a lookup matches no rows.
var ErrNotFound = errors.New("repository: not found")

// ErrConflict is returned when a unique constraint is violated.
var ErrConflict = errors.New("repository: conflict")
