package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The build-time half of the provenance chain lives in scripts/resolve-commit.sh:
// it decides, inside `docker build`, which revision gets stamped into the binary.
// Resolve() below can only report what that script found, so the script is tested
// here, next to it — a wrong answer at build time is indistinguishable at runtime
// from no answer at all.

// scriptPath locates the resolver relative to this package.
func scriptPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "resolve-commit.sh"))
	require.NoError(t, err)
	require.FileExists(t, path)
	return path
}

// runResolver executes the script against a throwaway build context and returns
// exactly what it wrote to stdout — no trimming, because stray whitespace would
// end up inside an -ldflags value.
func runResolver(t *testing.T, context string, env map[string]string) string {
	t.Helper()

	cmd := exec.Command("sh", scriptPath(t), context)
	cmd.Env = append(os.Environ(), "GIT_COMMIT=", "SOURCE_COMMIT=")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stderr []byte
	out, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr = exitErr.Stderr
	}
	require.NoError(t, err, "resolver failed: %s", stderr)
	return string(out)
}

// gitContext fabricates a build context whose .git holds the given files.
func gitContext(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return dir
}

const (
	argSHA    = "1111111111111111111111111111111111111111"
	sourceSHA = "2222222222222222222222222222222222222222"
	headSHA   = "3333333333333333333333333333333333333333"
)

func TestResolverPrefersTheExplicitBuildArg(t *testing.T) {
	context := gitContext(t, map[string]string{".git/HEAD": headSHA + "\n"})

	got := runResolver(t, context, map[string]string{
		"GIT_COMMIT":    argSHA,
		"SOURCE_COMMIT": sourceSHA,
	})

	assert.Equal(t, argSHA, got, "an explicit --build-arg GIT_COMMIT is the operator's word")
}

func TestResolverFallsBackToSourceCommit(t *testing.T) {
	context := gitContext(t, map[string]string{".git/HEAD": headSHA + "\n"})

	got := runResolver(t, context, map[string]string{"SOURCE_COMMIT": sourceSHA})

	assert.Equal(t, sourceSHA, got, "Coolify names the deployed revision SOURCE_COMMIT")
}

func TestResolverReadsADetachedHEAD(t *testing.T) {
	// How a deployment checkout actually looks: the CI or PaaS checks out one
	// commit, so HEAD holds the SHA itself.
	context := gitContext(t, map[string]string{".git/HEAD": headSHA + "\n"})

	got := runResolver(t, context, nil)

	assert.Equal(t, headSHA, got)
}

func TestResolverFollowsHEADToALooseRef(t *testing.T) {
	context := gitContext(t, map[string]string{
		".git/HEAD":            "ref: refs/heads/main\n",
		".git/refs/heads/main": headSHA + "\n",
	})

	got := runResolver(t, context, nil)

	assert.Equal(t, headSHA, got)
}

func TestResolverFollowsHEADIntoPackedRefs(t *testing.T) {
	// A freshly cloned repository packs its refs, so the loose file is absent.
	context := gitContext(t, map[string]string{
		".git/HEAD": "ref: refs/heads/main\n",
		".git/packed-refs": "# pack-refs with: peeled fully-peeled sorted \n" +
			sourceSHA + " refs/heads/other\n" +
			headSHA + " refs/heads/main\n" +
			argSHA + " refs/tags/v1\n",
	})

	got := runResolver(t, context, nil)

	assert.Equal(t, headSHA, got)
}

func TestResolverIsEmptyWithoutAnyGitMetadata(t *testing.T) {
	// A build context without .git — a tarball upload, or .dockerignore doing
	// its job. Nothing to report, and inventing something would be worse.
	got := runResolver(t, gitContext(t, nil), nil)

	assert.Equal(t, "", got)
}

func TestResolverIsEmptyWhenHEADPointsNowhere(t *testing.T) {
	context := gitContext(t, map[string]string{".git/HEAD": "ref: refs/heads/main\n"})

	got := runResolver(t, context, nil)

	assert.Equal(t, "", got, "an unresolvable ref is not a revision")
}

func TestResolverRejectsAValueThatIsNotASHA(t *testing.T) {
	// Guards the ldflags value: anything that is not a hex object name would be
	// shell noise at best, an injected linker flag at worst.
	for name, env := range map[string]map[string]string{
		"build arg":  {"GIT_COMMIT": "$(whoami)"},
		"source arg": {"SOURCE_COMMIT": "refs/heads/main"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "", runResolver(t, gitContext(t, nil), env))
		})
	}

	t.Run("corrupt HEAD", func(t *testing.T) {
		context := gitContext(t, map[string]string{".git/HEAD": "not a revision\n"})
		assert.Equal(t, "", runResolver(t, context, nil))
	})
}

func TestResolverEmitsTheSHAWithoutSurroundingWhitespace(t *testing.T) {
	context := gitContext(t, map[string]string{".git/HEAD": "  " + headSHA + "  \n"})

	got := runResolver(t, context, nil)

	assert.Equal(t, headSHA, got, "the value is substituted straight into -ldflags")
}

func TestResolverAcceptsAShortenedButValidRevision(t *testing.T) {
	// `make docker` on a stale tag, or a hand-passed abbreviation: still a
	// revision, still more useful than "unknown".
	got := runResolver(t, gitContext(t, nil), map[string]string{"GIT_COMMIT": "212b000"})

	assert.Equal(t, "212b000", got)
}
