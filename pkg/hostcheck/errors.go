package hostcheck

import "fmt"

type SkipHostCheckError struct {
	Node   string
	Reason string
}

func (e SkipHostCheckError) Error() string {
	return fmt.Sprintf("skip host checks on %s: %s", e.Node, e.Reason)
}
