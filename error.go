package services

import (
	"context"
	"net"
	"os"
	"strings"
	"syscall"
)

type errorCause interface {
	Cause() error
}

type errorTemporary interface {
	Temporary() bool
}

type errorTimeout interface {
	Timeout() bool
}

type errorUnreachable interface {
	Unreachable() bool
}

type wrappedError struct {
	cause error
}

func (e *wrappedError) Cause() error { return e.cause }

func (e *wrappedError) Error() string { return e.cause.Error() }

func (e *wrappedError) Canceled() bool { return isCanceled(e.cause) }

func (e *wrappedError) Timeout() bool { return isTimeout(e.cause) }

func (e *wrappedError) Temporary() bool { return isTemporary(e.cause) }

func (e *wrappedError) Unreachable() bool { return isUnreachable(e.cause) }

func (e *wrappedError) Validation() bool { return isValidation(e.cause) }

func wrapError(err error) error {
	if err == nil {
		return err
	}
	return &wrappedError{cause: err}
}

func isCanceled(err error) bool {
	if err != nil {
		switch e := err.(type) {
		case *net.DNSError:
			return isCanceledDNSError(e)
		case *net.OpError:
			return isCanceled(e.Err)
		case errorCause:
			return isCanceled(e.Cause())
		default:
			return err == context.Canceled
		}
	}
	return false
}

func isCanceledDNSError(e *net.DNSError) bool {
	return strings.HasSuffix(e.Err, ": operation was canceled")
}

func isTemporary(err error) bool {
	if err != nil {
		switch e := err.(type) {
		case *os.SyscallError:
			return isTemporary(e.Err)
		case errorTemporary:
			return e.Temporary()
		case errorCause:
			return isTemporary(e.Cause())
		}
	}
	return false
}

func isTimeout(err error) bool {
	if err != nil {
		switch e := err.(type) {
		case *os.SyscallError:
			return isTimeout(e.Err)
		case errorTimeout:
			return e.Timeout()
		case errorCause:
			return isTimeout(e.Cause())
		}
	}
	return false
}

func isUnreachable(err error) bool {
	if err != nil {
		switch e := err.(type) {
		case *net.DNSError:
			return isUnreachableDNSError(e)
		case *net.OpError:
			return isUnreachable(e.Err)
		case *os.SyscallError:
			return isUnreachable(e.Err)
		case syscall.Errno:
			return isUnreachableErrno(e)
		case errorUnreachable:
			return e.Unreachable()
		case errorCause:
			return isUnreachable(e.Cause())
		}
	}
	return false
}

func isUnreachableDNSError(e *net.DNSError) bool {
	// https://golang.org/src/net/net.go?h="no+such+host"
	return e.Err == "no such host"
}

func isUnreachableErrno(e syscall.Errno) bool {
	return e == syscall.ECONNABORTED || e == syscall.ECONNREFUSED || e == syscall.ECONNRESET
}

func isValidation(err error) bool {
	if err != nil {
		switch e := err.(type) {
		case *net.AddrError:
			return isValidationError(e.Err)
		case *net.DNSError:
			return isValidationError(e.Err)
		case *net.ParseError:
			return true
		case *net.OpError:
			return isValidation(e.Err)
		case *os.SyscallError:
			return isValidation(e.Err)
		case syscall.Errno:
			return isValidationErrno(e)
		case errorCause:
			return isValidation(e.Cause())
		default:
			return isValidationError(e.Error())
		}
	}
	return false
}

func isValidationErrno(e syscall.Errno) bool {
	return e == syscall.EINVAL || e == syscall.EPERM
}

func isValidationError(s string) bool {
	// according to https://golang.org/search?q=AddrError%7B, those are
	// common prefixes of net errors for invalid input.
	return strings.HasPrefix(s, "mismatched ") ||
		strings.HasPrefix(s, "unexpected ") ||
		strings.HasPrefix(s, "invalid ") ||
		strings.HasPrefix(s, "unknown ") ||
		strings.HasPrefix(s, "missing")
}
