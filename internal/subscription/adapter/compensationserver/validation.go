package compensationserver

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

func newValidationInterceptor(validator protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok {
				if err := validator.Validate(msg); err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(validationMessage(err)))
				}
			}

			return next(ctx, req)
		}
	}
}

func validationMessage(err error) string {
	if ve, ok := errors.AsType[*protovalidate.ValidationError](err); ok && len(ve.Violations) > 0 {
		if fd := ve.Violations[0].FieldDescriptor; fd != nil {
			switch string(fd.Name()) {
			case "saga_id":
				return "invalid saga id, expected a uuid"
			case "saga_type":
				return "saga type must not be empty"
			case "email":
				return "invalid email format"
			case "repo_name":
				return "invalid repo name, expected owner/repo"
			}
		}
	}

	return "invalid request"
}
