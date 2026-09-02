package plugin

import (
	"fmt"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
	"net/http"
)

var ErrNilComponent = fmt.Errorf("plugin pipeline contains nil component")

type Context struct {
	Request *http.Request
	Route   router.Route
}

type Outcome int

const (
	Continue Outcome = iota
	Reject
)

type Result struct {
	Outcome    Outcome
	StatusCode int
	Message    string
}

type Plugin interface {
	Name() string
	Execute(*Context) (Result, error)
}

type Policy interface {
	Name() string
	Evaluate(*Context) (Result, error)
}

type component interface {
	Name() string
	Execute(*Context) (Result, error)
}

type pluginComponent struct {
	plugin Plugin
}

func (c pluginComponent) Name() string {
	return c.plugin.Name()
}

func (c pluginComponent) Execute(ctx *Context) (Result, error) {
	return c.plugin.Execute(ctx)
}

type policyComponent struct {
	policy Policy
}

func (c policyComponent) Name() string {
	return c.policy.Name()
}

func (c policyComponent) Execute(ctx *Context) (Result, error) {
	return c.policy.Evaluate(ctx)
}

type Pipeline struct {
	components []component
}

func NewPipeline(components ...component) *Pipeline {
	return &Pipeline{
		components: components,
	}
}

func PluginComponent(plugin Plugin) component {
	return pluginComponent{
		plugin: plugin,
	}
}

func PolicyComponent(policy Policy) component {
	return policyComponent{
		policy: policy,
	}
}

func (p *Pipeline) Execute(ctx *Context) (Result, error) {
	for _, component := range p.components {
		if component == nil {
			return Result{}, ErrNilComponent
		}

		result, err := component.Execute(ctx)
		if err != nil {
			return Result{}, err
		}

		if result.Outcome == Reject {
			return result, nil
		}
	}

	return Result{
		Outcome: Continue,
	}, nil
}
