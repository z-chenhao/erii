// Package app is the composition root. Readiness and persistence belong to agent;
// process configuration and telemetry belong to their respective modules.
package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/agent/sqlite"
	artifacttools "github.com/z-chenhao/erii/internal/artifact/tools"
	computerfake "github.com/z-chenhao/erii/internal/computer/fake"
	"github.com/z-chenhao/erii/internal/computer/workspace"
	"github.com/z-chenhao/erii/internal/config"
	"github.com/z-chenhao/erii/internal/conversation"
	conversationsqlite "github.com/z-chenhao/erii/internal/conversation/sqlite"
	"github.com/z-chenhao/erii/internal/delivery"
	deliveryfake "github.com/z-chenhao/erii/internal/delivery/fake"
	deliveryterminal "github.com/z-chenhao/erii/internal/delivery/terminal"
	"github.com/z-chenhao/erii/internal/event"
	eventnats "github.com/z-chenhao/erii/internal/event/nats"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/llm/account"
	"github.com/z-chenhao/erii/internal/llm/bridge"
	"github.com/z-chenhao/erii/internal/llm/fake"
	"github.com/z-chenhao/erii/internal/llm/secret"
	llmsqlite "github.com/z-chenhao/erii/internal/llm/sqlite"
	"github.com/z-chenhao/erii/internal/observability"
	"github.com/z-chenhao/erii/internal/skill"
	"github.com/z-chenhao/erii/internal/task"
	"github.com/z-chenhao/erii/internal/tool"
	"github.com/z-chenhao/erii/internal/tool/codemode"
	toolfake "github.com/z-chenhao/erii/internal/tool/fake"
	"github.com/z-chenhao/erii/internal/tool/mcp"
	"github.com/z-chenhao/erii/internal/tui"
)

type App struct {
	store          *sqlite.Store
	artifacts      artifactStore
	telemetry      *observability.Telemetry
	provider       *bridge.Client
	control        llm.Control
	accounts       *account.Manager
	history        *conversationsqlite.Store
	runs           *sqlite.Runs
	registry       *tool.Registry
	terminalInbox  *deliveryterminal.Adapter
	deliveryPolicy delivery.Policy
	deliveryFake   *deliveryfake.Adapter
	testDelivery   string
	testBoundary   func(string)
}

func Open(ctx context.Context, cfg config.Config) (*App, error) {
	return open(ctx, cfg, "", secret.Store{})
}

func OpenWithTestSecrets(ctx context.Context, cfg config.Config, path string) (*App, error) {
	return open(ctx, cfg, "", secret.TestSocket{Path: path})
}

// OpenWithTestBridge is explicit test injection. No config default, environment
// discovery, missing credential, or saved active profile can select a Fake.
func OpenWithTestBridge(ctx context.Context, cfg config.Config, fixture string) (*App, error) {
	if !fake.ValidFixture(fixture) {
		return nil, llm.InvalidProviderResponse
	}
	return open(ctx, cfg, fixture, secret.Store{})
}

func open(ctx context.Context, cfg config.Config, fixture string, secrets llm.SecretStore) (*App, error) {
	t, err := observability.Open(cfg.App.DataRoot, cfg.LogLevel())
	if err != nil {
		return nil, err
	}
	ctx, startup, err := t.Start(ctx, "app.startup")
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	_, database, err := t.Start(ctx, "sqlite.open_and_migrate")
	if err != nil {
		startup.End(err)
		return nil, errors.Join(err, t.Close())
	}
	store, err := sqlite.Open(ctx, cfg.App.DataRoot)
	database.End(err)
	if err != nil {
		startup.End(err)
		return nil, errors.Join(err, t.Close())
	}
	store.Artifacts().Policy = cfg.Artifact
	a := &App{store: store, telemetry: t, artifacts: store.Artifacts()}
	a.history, err = conversationsqlite.Open(ctx, cfg.App.DataRoot)
	if err != nil {
		_ = store.Close()
		_ = t.Close()
		return nil, err
	}
	a.terminalInbox = store.TerminalInbox()
	a.deliveryPolicy = cfg.Delivery
	base := task.Registry()
	defs := append(artifacttools.Definitions(), agent.AgentToolDefinition())
	if cfg.JavaScript != nil {
		runtime := &codemode.Sidecar{Command: cfg.JavaScript.Command, Script: cfg.JavaScript.Script}
		runtimeHash, preludeHash, e := runtime.Fingerprint()
		if e != nil {
			a.Close()
			return nil, e
		}
		definition := codemode.Definition()
		definition.Spec.ResourceHash += ":" + runtimeHash + ":" + preludeHash
		defs = append(defs, definition)
	}
	for _, server := range cfg.MCP {
		more, e := mcp.Definitions(ctx, server)
		if e != nil {
			a.Close()
			return nil, e
		}
		defs = append(defs, more...)
	}
	if cfg.Skills != nil {
		more, e := skill.Definitions(*cfg.Skills)
		if e != nil {
			a.Close()
			return nil, e
		}
		defs = append(defs, more...)
	}
	if cfg.Workspace != nil {
		more, e := workspace.Definitions(*cfg.Workspace)
		if e != nil {
			a.Close()
			return nil, e
		}
		defs = append(defs, more...)
	}
	if fixture == "codemode" && cfg.JavaScript != nil {
		defs = append(defs, computerfake.Definitions()...)
	}
	if fixture == "artifacts" {
		defs = append(defs, toolfake.Artifacts()...)
	}
	for _, spec := range base.Describe() {
		d, _ := base.Resolve(spec.Name)
		defs = append(defs, d)
	}
	a.registry, err = tool.NewRegistry(defs)
	if err != nil {
		a.Close()
		return nil, err
	}
	if fixture == "codemode" || fixture == "children" || fixture == "approval" || fixture == "approval-never" {
		defs := []tool.Definition{}
		for _, spec := range a.registry.Describe() {
			d, _ := a.registry.Resolve(spec.Name)
			defs = append(defs, d)
		}
		policy := "query_before_retry"
		if fixture == "approval-never" {
			policy = "never"
		}
		defs = append(defs, toolfake.Definition(cfg.App.DataRoot, policy, func(name string) {
			if a.testBoundary != nil {
				a.testBoundary(name)
			}
		}))
		a.registry, err = tool.NewRegistry(defs)
		if err != nil {
			a.Close()
			return nil, err
		}
	}
	a.runs = store.Runs(a.history, a.registry)
	if cfg.JavaScript != nil {
		a.runs.ConfigureJavaScript(&codemode.Sidecar{Command: cfg.JavaScript.Command, Script: cfg.JavaScript.Script})
	}
	a.runs.ConfigureChildLimits(cfg.Children)
	a.runs.ConfigureScheduler(cfg.Scheduler)
	a.runs.ConfigureSpend(cfg.Spend)
	a.runs.ConfigureApprovalTTL(time.Duration(cfg.Tools.ApprovalTTLSeconds) * time.Second)
	a.runs.ConfigureToolLimits(sqlite.ToolLimits{Global: cfg.Tools.Global, Agent: cfg.Tools.Agent, Run: cfg.Tools.Run, Provider: cfg.Tools.Provider, Tool: cfg.Tools.Tool})
	a.runs.ConfigureRecentTasks(time.Duration(cfg.Tasks.RecentDays)*24*time.Hour, cfg.Tasks.RecentLimit)
	a.control = llm.Control{Store: store.Models(), Secrets: secrets}
	a.accounts = &account.Manager{Store: store.Models(), Secrets: secrets, LockRoot: filepath.Join(cfg.App.DataRoot, "auth-locks")}
	if fixture != "" {
		path, err := os.Executable()
		if err == nil {
			a.provider, err = bridge.OpenObserved(ctx, path, a.accounts.Observer(a.recordGeneration), "-test-provider-bridge", fixture, filepath.Join(cfg.App.DataRoot, "fake-provider-calls.jsonl"))
		}
		if err != nil {
			a.control.BridgeError = llm.NormalizeError(err.Error())
		} else {
			a.control.Probe = a.provider
			a.control.BridgeIdentity = "test-bridge-v1:" + fixture
		}
	}
	if fixture == "" && cfg.LLM.ProviderBridgeCommand != "" {
		a.provider, err = bridge.OpenLocal(ctx, cfg.LLM.ProviderBridgeCommand, cfg.LLM.ProviderBridgeScript, a.accounts.Resolver(store.Models().Resolver(secrets)), a.accounts.Observer(a.recordGeneration))
		if err != nil {
			a.control.BridgeError = llm.NormalizeError(err.Error())
		} else {
			a.control.Probe = a.provider
			a.control.BridgeIdentity = cfg.LLM.ProviderBridgeVersion
		}
	}
	if fixture == "" {
		if a.provider != nil {
			a.accounts.Provider = a.provider
		}
		a.control.AccountAuth = a.accounts.Check
		if err = a.accounts.Recover(ctx); err != nil {
			_ = a.Close()
			return nil, err
		}
	}
	startup.Logger.Info("application started")
	startup.End(nil)
	return a, nil
}

func (a *App) Diagnostics(ctx context.Context) (agent.Diagnostics, error) {
	d, err := a.store.Diagnostics(ctx)
	if err != nil {
		return d, err
	}
	return agent.EffectiveReadiness(ctx, d, a.control, a.provider), nil
}

func (a *App) Models() *llmsqlite.Store   { return a.store.Models() }
func (a *App) ModelControl() llm.Control  { return a.control }
func (a *App) Accounts() *account.Manager { return a.accounts }
func (a *App) Catalog(ctx context.Context) ([]llm.CatalogModel, error) {
	if a.provider == nil {
		if a.control.BridgeError != nil {
			return nil, a.control.BridgeError
		}
		return nil, llm.ErrNoBridge
	}
	return a.provider.List(ctx)
}

func (a *App) Close() error {
	if a.provider != nil {
		_ = a.provider.Close()
	}
	err := a.store.Close()
	err = errors.Join(err, a.history.Close())
	a.telemetry.Logger.Info("application stopped")
	return errors.Join(err, a.telemetry.Close())
}

func (a *App) recordGeneration(ctx context.Context, r llm.GenerationRequest, result llm.GenerationResult, latency time.Duration, cause error) error {
	err := a.store.Models().RecordUsageAnd(ctx, r, result, latency, cause, func(ctx context.Context, tx *sql.Tx, r llm.GenerationRequest, result llm.GenerationResult, cause error) error {
		return a.store.AcceptGeneration(ctx, tx, r, result, cause)
	})
	if err == nil && r.OwnerToken != "" && a.store.MainBoundary != nil {
		a.store.MainBoundary("after_generation_commit")
	}
	return err
}

func (a *App) SetTestMainBoundary(boundary func(string)) {
	a.store.MainBoundary = boundary
	a.testBoundary = boundary
}
func (a *App) Actor(sink agent.EventSink) agent.Actor {
	var provider llm.LLMProvider
	if a.provider != nil {
		provider = a.provider
	}
	return agent.Actor{Store: a.runs, Provider: provider, Registry: a.registry, Sink: sink, Ready: func(ctx context.Context) bool {
		d, err := a.Diagnostics(ctx)
		return err == nil && d.AgentIngressEnabled
	}}
}
func (a *App) History(ctx context.Context) ([]conversation.Message, error) {
	if err := a.runs.Flush(ctx); err != nil {
		return nil, err
	}
	d, err := a.store.Diagnostics(ctx)
	if err != nil {
		return nil, err
	}
	return a.history.List(ctx, d.AgentID)
}

func (a *App) Terminal() tui.Session {
	return tui.Session{State: a, Models: a.Models(), Accounts: a.accounts, DecideApproval: a.DecideApproval,
		Messages: a.TerminalHistory, Control: a.control, Actor: a.Actor(nil), Discover: a.Discover,
		AccountCatalog: a.AccountCatalog, LoginEnvironment: a.LoginEnvironment}
}

// Relay runs infrastructure only. It never enables Agent ingress or a model.
func (a *App) Relay(ctx context.Context, cfg config.Config) error {
	return event.Relay(ctx, a.store.Events(), func(ctx context.Context) (event.EventBus, error) {
		bus, err := eventnats.Open(ctx, cfg.NATS)
		if err != nil {
			return nil, err
		}
		return bus, nil
	}, cfg.NATS.RetryInterval, a.telemetry.Logger)
}

func (a *App) CatalogProfile(ctx context.Context, id string) ([]llm.CatalogModel, error) {
	if a.provider == nil {
		return nil, llm.ErrNoBridge
	}
	p, err := a.Models().Profile(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.ProviderKind == "openai_account" {
		return a.AccountCatalog(ctx, p.AuthConnectionID)
	}
	return a.provider.CatalogForProfile(ctx, p)
}

func (a *App) AccountCatalog(ctx context.Context, id string) ([]llm.CatalogModel, error) {
	if a.provider == nil {
		return nil, llm.ErrNoBridge
	}
	return a.accounts.Catalog(ctx, id, a.provider.CatalogAt)
}

// LoginKey consumes only stdin bytes; credentials never enter arguments or metadata.
func (a *App) LoginKey(ctx context.Context, value []byte) (llm.AuthConnection, error) {
	return llm.RegisterAuth(ctx, a.Models(), a.control.Secrets, llm.AuthInput{Provider: "openai_compatible", AuthKind: "api_key"}, value)
}
func (a *App) LoginEnvironment(ctx context.Context, name string) (llm.AuthConnection, error) {
	return secret.RegisterEnvironment(ctx, a.Models(), name)
}

func (a *App) Discover(ctx context.Context, endpoint, authID string) ([]llm.CatalogModel, error) {
	if a.provider == nil {
		return nil, llm.ErrNoBridge
	}
	connection, err := a.Models().Connection(ctx, endpoint, authID, a.control.Secrets)
	if err != nil {
		return nil, err
	}
	return a.provider.CatalogAt(ctx, connection)
}

func (a *App) Activity(ctx context.Context) ([]task.Summary, error) { return a.runs.Activity(ctx) }

func (a *App) Spend(ctx context.Context) ([]sqlite.SpendView, error) { return a.runs.Spend(ctx) }

func (a *App) Approvals(ctx context.Context) ([]sqlite.ApprovalView, error) {
	return a.runs.Approvals(ctx)
}
func (a *App) DecideApproval(ctx context.Context, id, hash, decision string) error {
	d, err := a.store.Diagnostics(ctx)
	if err != nil {
		return err
	}
	return a.runs.Decide(ctx, d.AgentID, id, hash, decision)
}
func (a *App) ReconcileTools(ctx context.Context) ([]sqlite.ToolOutcome, error) {
	if err := a.runs.ReconcileUnknown(ctx); err != nil {
		return nil, err
	}
	return a.runs.ToolOutcomes(ctx)
}
