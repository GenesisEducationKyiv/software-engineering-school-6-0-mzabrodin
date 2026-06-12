package connectapi

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

func NewValidationInterceptor(validator protovalidate.Validator) connect.UnaryInterceptorFunc {
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
			case "email":
				return "invalid email format"
			case "repo":
				return "invalid repo format, expected owner/repo"
			case "token":
				return "invalid token"
			}
		}
	}

	return "invalid request"
}
