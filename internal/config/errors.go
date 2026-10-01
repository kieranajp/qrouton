package config

import "errors"

var (
	ErrThoughtsRoot      = errors.New(`each extra thoughts root needs its own id (not "default") and at least one org`)
	ErrThoughtsOrg       = errors.New("an org maps to two thoughts roots")
	ErrThoughtsHoldsRoot = errors.New("a thoughts folder must not be or contain the sessions root")
)
