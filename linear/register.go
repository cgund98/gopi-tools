package linear

import (
	"fmt"
	"os"
	"sync"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// toolSpec builds one tool from shared state. hasRead reports whether the tool
// has at least one read operation, and so is worth registering read-only.
type toolSpec struct {
	build   func(s *shared, readonly bool) gogent.Tool
	hasRead bool
}

// toolSpecs is the single source of truth for which tools exist. Tools and
// Options both walk it, so a new tool is added in exactly one place.
var toolSpecs = []toolSpec{
	{build: newIssuesTool, hasRead: true},
	{build: newProjectsTool, hasRead: true},
	{build: newCommentsTool},
	{build: newTeamsTool, hasRead: true},
	{build: newUsersTool, hasRead: true},
	{build: newLabelsTool, hasRead: true},
}

// Tools returns every Linear tool. They share one client, resolver, and name
// cache, so a name one tool records is visible when another tool renders.
func Tools(cfg Config) []gogent.Tool {
	s := newShared(cfg)
	tools := make([]gogent.Tool, 0, len(toolSpecs))
	for _, spec := range toolSpecs {
		tools = append(tools, spec.build(s, false))
	}
	return tools
}

// Options returns the gopi options that register the Linear tools. Every tool
// goes in agent mode. A tool with a read operation also goes in plan and ask
// mode in its read-only form, so gopi can look tickets up while planning with
// no chance of a write.
//
// The shared state is built once, from the first ToolEnv gopi supplies, and
// every factory closes over it.
func Options() []gopi.Option {
	state := &lazyShared{}
	var opts []gopi.Option
	for _, spec := range toolSpecs {
		opts = append(opts, modeOption(gopi.ModeAgent, state, spec, false))
		if spec.hasRead {
			opts = append(opts, modeOption(gopi.ModePlan, state, spec, true))
			opts = append(opts, modeOption(gopi.ModeAsk, state, spec, true))
		}
	}
	return opts
}

// modeOption builds one WithToolFactory for one mode.
func modeOption(mode gopi.Mode, state *lazyShared, spec toolSpec, readonly bool) gopi.Option {
	return gopi.WithToolFactory(mode, func(env gopi.ToolEnv) (gogent.Tool, error) {
		s, err := state.get(env)
		if err != nil {
			return nil, err
		}
		return spec.build(s, readonly), nil
	})
}

// lazyShared builds the shared state once, on first use. gopi calls one factory
// per registered tool, and they must all share the same state.
type lazyShared struct {
	once sync.Once
	s    *shared
	err  error
}

func (l *lazyShared) get(env gopi.ToolEnv) (*shared, error) {
	l.once.Do(func() {
		cfg, err := configFromEnv(env)
		if err != nil {
			l.err = err
			return
		}
		l.s = newShared(cfg)
	})
	if l.err != nil {
		return nil, l.err
	}
	return l.s, nil
}

// linearSettings is the [linear] table in ~/.gopi/config.toml.
type linearSettings struct {
	// DefaultTeam is a team key (such as "ENG"), name, or UUID used whenever a
	// Linear operation omits the team.
	DefaultTeam string `toml:"default_team"`
}

// configFromEnv reads the [linear] table and the linear_api_key secret.
func configFromEnv(env gopi.ToolEnv) (Config, error) {
	var settings linearSettings
	if err := env.Config("linear", &settings); err != nil {
		return Config{}, err
	}
	apiKey, err := secretOrEnv(env, "linear_api_key", "LINEAR_API_KEY")
	if err != nil {
		return Config{}, err
	}
	return Config{APIKey: apiKey, DefaultTeam: settings.DefaultTeam}, nil
}

// secretOrEnv reads a secret, falling back to an environment variable.
func secretOrEnv(env gopi.ToolEnv, name, envName string) (string, error) {
	if value, err := env.Secret(name); err == nil {
		return value, nil
	}
	if value := os.Getenv(envName); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("set %s in ~/.gopi/secrets.toml or %s in the environment", name, envName)
}
