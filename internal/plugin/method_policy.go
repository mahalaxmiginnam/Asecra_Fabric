package plugin

import "net/http"

type MethodPolicy struct {
	AllowedMethods map[string]bool
}

func (p MethodPolicy) Name() string {
	return "method-policy"
}

func (p MethodPolicy) Evaluate(ctx *Context) (Result, error) {
	if ctx == nil || ctx.Request == nil {
		return Result{
			Outcome:    Reject,
			StatusCode: http.StatusBadRequest,
			Message:    "request context unavailable",
		}, nil
	}

	if p.AllowedMethods[ctx.Request.Method] {
		return Result{
			Outcome: Continue,
		}, nil
	}

	return Result{
		Outcome:    Reject,
		StatusCode: http.StatusMethodNotAllowed,
		Message:    "method not allowed",
	}, nil
}
