package app

import "errors"

// ErrConfig marks an error caused by an invalid configuration. Test for it
// with errors.Is; callers decide what exit status (if any) that maps to.
var ErrConfig = errors.New("invalid configuration")

type configError struct{ err error }

func (e *configError) Error() string   { return e.err.Error() }
func (e *configError) Unwrap() []error { return []error{e.err, ErrConfig} }

// configErr tags err as a configuration error.
func configErr(err error) error { return &configError{err} }

type reportedError struct{ err error }

func (e *reportedError) Error() string { return e.err.Error() }
func (e *reportedError) Unwrap() error { return e.err }

// reported marks an error whose message has already been written to the log
// and stderr, so the caller must not print it again.
func reported(err error) error { return &reportedError{err} }

// IsReported reports whether err was already shown to the user.
func IsReported(err error) bool {
	var r *reportedError
	return errors.As(err, &r)
}
