package auth

import (
	"fmt"

	"github.com/casbin/casbin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Accepts ACL model and policy files
func New(model, policy string) *Authorizer {
	enforcer := casbin.NewEnforcer(model, policy)
	return &Authorizer{enforcer}
}

type Authorizer struct {
	enforcer *casbin.Enforcer
}

func (this *Authorizer) Authorize(subject, object, action string) error {
	if !this.enforcer.Enforce(subject, object, action) {
		msg := fmt.Sprintf(
			"%s not permitted to %s to %s",
			subject,
			action,
			object,
		)
		st := status.New(codes.PermissionDenied, msg)
		return st.Err()
	}

	return nil
}
