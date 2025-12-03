package models

import "errors"

// Repository errors
var (
	ErrClusterNotFound                = errors.New("cluster not found")
	ErrNodePoolNotFound               = errors.New("nodepool not found")
	ErrReconciliationScheduleNotFound = errors.New("reconciliation schedule not found")
	ErrInvalidInput                   = errors.New("invalid input")
	ErrConflict                       = errors.New("resource conflict")
	ErrDuplicateEntry                 = errors.New("duplicate entry")
)

// ListOptions represents common filtering and pagination options
type ListOptions struct {
	Status string `json:"status,omitempty"`
	Health string `json:"health,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Validate validates the list options
func (opts *ListOptions) Validate() error {
	if opts.Limit < 0 {
		return ErrInvalidInput
	}
	if opts.Offset < 0 {
		return ErrInvalidInput
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000 // Cap at 1000 for performance
	}
	if opts.Limit == 0 {
		opts.Limit = 50 // Default limit
	}
	return nil
}
