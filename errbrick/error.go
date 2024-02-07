package errbrick

import "errors"

var (
	// ErrInvalidData is a generic error that should be used when the input data is invalid.
	// It should be used with a more specific error message.
	// Example: fmt.Errorf("%w: %s", errbrick.ErrInvalidData, "invalid print date")
	// So, the error message will be: "invalid data: invalid print date", the error type will be errbrick.ErrInvalidData
	// and the caller can check for it with errors.Is(err, errbrick.ErrInvalidData).
	ErrInvalidData = errors.New("invalid data")

	// ErrNotFound is a generic error that should be used when the requested resource is not found.
	// It should be used with a more specific error message.
	// Example: fmt.Errorf("failed to fetch user %d: %w", 123, errbrick.ErrNotFound)
	// So, the error message will be: "failed to fetch user 123: not found", the error type will be errbrick.ErrNotFound
	// and the caller can check for it with errors.Is(err, errbrick.ErrInvalidData).
	ErrNotFound = errors.New("not found")

	// ErrConflict is a generic error that should be used when the requested resource already exists or some another data conflict occurs.
	// It should be used with a more specific error message.
	// Example: fmt.Errorf("%w: %s", errbrick.ErrConflict, "user already exist")
	// So, the error message will be: "conflict: user already exist", the error type will be errbrick.ErrConflict
	// and the caller can check for it with errors.Is(err, errbrick.ErrConflict).
	ErrConflict = errors.New("conflict")

	// ErrForbidden is a generic error that should be used when the subject is not correctly authorized.
	// It should be used with a more specific error message.
	// Example: fmt.Errorf("%w: %s", errbrick.ErrForbidden, "only admins can modify the data")
	// So, the error message will be: "forbidden: only admins can modify the data", the error type will be errbrick.ErrForbidden
	// and the caller can check for it with errors.Is(err, errbrick.ErrForbidden).
	ErrForbidden = errors.New("forbidden")

	// ErrUnauthenticated is a generic error that should be used when the subject is not authenticated.
	// It should be used with a more specific error message.
	// Example: fmt.Errorf("%w: %s", errbrick.ErrUnauthenticated, "invalid token")
	// So, the error message will be: "unauthenticated: invalid token", the error type will be errbrick.ErrUnauthenticated
	// and the caller can check for it with errors.Is(err, errbrick.ErrUnauthenticated).
	ErrUnauthenticated = errors.New("unauthenticated")
)

var errList = []error{
	ErrInvalidData,
	ErrNotFound,
	ErrConflict,
	ErrForbidden,
	ErrUnauthenticated,
}

func IsOneOf(err error, errs ...error) bool {
	if err == nil {
		return false
	}
	if len(errs) == 0 {
		errs = append(errs, errList...)
	}
	for _, e := range errs {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
