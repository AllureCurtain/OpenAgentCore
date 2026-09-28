package localworkspace

import "errors"

// InitializationFailure contains only a confirmed step's safe exit status.
type InitializationFailure struct{ ExitCode *int }

func (*InitializationFailure) Error() string { return "Runtime initialization failed" }

// ErrInitializationUnconfirmed means execution or its receipt was not confirmed.
var ErrInitializationUnconfirmed = errors.New("Runtime initialization unconfirmed")
