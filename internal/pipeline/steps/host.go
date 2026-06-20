package steps

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/kunchenguid/no-mistakes/internal/bitbucket"
	"github.com/kunchenguid/no-mistakes/internal/pipeline"
	"github.com/kunchenguid/no-mistakes/internal/scm"
	"github.com/kunchenguid/no-mistakes/internal/scm/github"
	"github.com/kunchenguid/no-mistakes/internal/scm/gitlab"
)

// buildHost returns a scm.Host for the given provider, wired to sctx's
// working directory and environment. When the host cannot be constructed
// (unknown provider, missing Bitbucket config, etc) it returns nil and a
// human-readable skip reason suitable for logging.
func buildHost(sctx *pipeline.StepContext, provider scm.Provider) (scm.Host, string) {
	cmdFactory := func(_ context.Context, name string, args ...string) *exec.Cmd {
		return stepCmd(sctx, name, args...)
	}
	switch provider {
	case scm.ProviderGitHub:
		// Resolve the parent owner/name slug so gh commands carry --repo and
		// work from the daemon's fixed (non-repo) working directory. Fall back
		// to the PR URL when the upstream remote URL is unavailable.
		repo := github.RepoSlug(sctx.Repo.UpstreamURL)
		if repo == "" && sctx.Run.PRURL != nil {
			repo = github.RepoSlug(*sctx.Run.PRURL)
		}
		// For fork contributions, also resolve the fork slug so PR creation
		// emits --head "<fork_owner>:<branch>" against the parent (--repo).
		forkRepo := github.RepoSlug(sctx.Repo.ForkURL)
		return github.NewWithFork(cmdFactory, func() bool { return stepCLIAvailable(sctx, provider) }, repo, forkRepo), ""
	case scm.ProviderGitLab:
		// github.RepoSlug parses any owner/name remote URL regardless of host,
		// so it works for GitLab fork URLs too. glab resolves the MR project
		// from the working repo rather than a flag, so this only labels the
		// fork identity for now; see gitlab.NewWithFork.
		forkProject := github.RepoSlug(sctx.Repo.ForkURL)
		return gitlab.NewWithFork(cmdFactory, func() bool { return stepCLIAvailable(sctx, provider) }, forkProject), ""
	case scm.ProviderBitbucket:
		client, err := bitbucket.NewClientFromEnv(sctx.Env)
		if err != nil {
			return nil, err.Error()
		}
		repo, err := resolveBitbucketRepoRef(sctx.Repo.UpstreamURL, sctx.Run.PRURL)
		if err != nil {
			return nil, err.Error()
		}
		forkRepo, _ := resolveBitbucketRepoRef(sctx.Repo.ForkURL, nil)
		return bitbucket.NewHostWithFork(client, repo, forkRepo), ""
	default:
		return nil, fmt.Sprintf("provider %s is not supported yet", provider)
	}
}
