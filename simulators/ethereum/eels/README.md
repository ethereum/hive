# EELS simulators

The `ethereum/eels/*` simulators run the `consume` and `execute` commands of
[ethereum/execution-specs](https://github.com/ethereum/execution-specs) against
clients. Each simulator's default Dockerfile starts from a pre-built image
published by that repository under `ghcr.io/ethereum/execution-specs/hive/`.
Nothing is cloned or downloaded when hive builds the simulator image; hive pulls
one image and adds a few metadata layers. `Dockerfile.git` in each directory
builds the simulator from source instead, see
[Building from source](#building-from-source).

## What a tag selects

**A tag selects the version of the tests. The simulator and framework code
follows the head of the branch the tests belong to.** A `consume` simulator
carries the release's generated fixtures; an `execute` simulator carries the
release's Python test sources, snapshotted at the commit that produced it. In
both, the framework, exception mappings included, is the branch head, so runner
fixes arrive without a new test release, and `consume-engine` and
`execute-blobs` under the same tag run the same tests.

| Simulator | Image | Test content in the image |
| --- | --- | --- |
| `consume-rlp` | `ghcr.io/ethereum/execution-specs/hive/consume-rlp` | `blockchain_tests` fixtures |
| `consume-engine` | `ghcr.io/ethereum/execution-specs/hive/consume-engine` | `blockchain_tests_engine` fixtures |
| `consume-enginex` | `ghcr.io/ethereum/execution-specs/hive/consume-enginex` | `blockchain_tests_engine_x` fixtures |
| `consume-sync` | `ghcr.io/ethereum/execution-specs/hive/consume-sync` | `blockchain_tests_sync` fixtures |
| `execute-blobs` | `ghcr.io/ethereum/execution-specs/hive/execute-blobs` | the test sources of the release |

## Build arguments

| Argument | Default | Purpose |
| --- | --- | --- |
| `tag` | `latest` | Selects the tests, see below. `<tag>@sha256:<digest>` pins an exact image. |
| `image` | the image in the table above | Only needed to use another registry namespace or a locally built image. |
| `disable_strict_exception_matching` | `nimbus-el` | `consume-engine` and `consume-enginex` only; an empty value exempts no client. |
| `fork` | `Osaka` | `execute-blobs` only. |

## Tags

A tag is an execution-specs release name, the one `consume --input` accepts,
with `@` replaced by `-`, or a channel that follows the releases of a line.
Examples: release 8.1.4 of the Glamsterdam devnet, cut from
`devnets/glamsterdam/8`, and mainnet release 20.0.2.

| Tag | Devnet example | Mainnet example | Meaning |
| --- | --- | --- | --- |
| release | `glamsterdam-devnet-v8.1.4` | `v20.0.2` | that release's tests; the framework follows the branch head while the release is the branch's current one |
| branch channel | `glamsterdam-devnet-8` | `latest` | the branch's highest release with the branch's head; what a dashboard for one devnet follows |
| series channel | `glamsterdam-devnet-latest` | `latest` | the highest release of the series; moves to devnet 9 with its first release |
| nightly | | `nightly`, `nightly-<commit>` | the most recent nightly fill of the main line: fixtures, test sources and framework from one commit |

`latest` is what `consume --input tests@latest` resolves, `glamsterdam-devnet-8`
is to devnet 8 what `latest` is to the default branch, and no tag is named after
a fork. Channel tags are rebuilt whenever their branch moves; a release tag
keeps the framework of its last build once a newer release of its branch
exists. Tags name sources, not bytes: to run the exact image of a previous run,
append its digest to the tag, `tag=latest@sha256:<digest>`. The simulator log
header names the sources of every run as `consume ref` or `execute ref` and
`fixtures release` or `tests release`.

## Building from source

`Dockerfile.git` in each simulator directory is the previous build: it clones
execution-specs, runs `uv sync` and, for the `consume-*` simulators, downloads a
fixture release with `consume cache`. Select it with `dockerfile: git` in a
`--sim.file` configuration. It takes `branch`, an execution-specs Git ref that
defaults to the repository's default branch, and `fixtures`, the `consume
--input` value that defaults to `stable@latest`. The release name behind an
image tag, `tests-glamsterdam-devnet@v8.1.4` for `glamsterdam-devnet-v8.1.4`, is
the `fixtures` input that gives the source build the same tests, and a commit
from a simulator log header as `branch` reproduces a run's sources from source:

```yaml
- simulator: ethereum/eels/consume-rlp
  dockerfile: git
  build_args:
    branch: devnets/glamsterdam/8
    fixtures: tests-glamsterdam-devnet@v8.1.4
```

The simulator build parameters section of [docs/commandline.md] pairs each tag
with its release name and branch.

## Examples

```sh
# latest mainnet release, framework at the head of the default branch
./hive --sim ethereum/eels/consume-engine --client go-ethereum

# follow the glamsterdam devnet 8 releases, as a dashboard for that devnet does
./hive --sim ethereum/eels/consume-engine --client go-ethereum --sim.buildarg tag=glamsterdam-devnet-8

# last night's fill of the main line
./hive --sim ethereum/eels/consume-engine --client go-ethereum --sim.buildarg tag=nightly

# a pinned release
./hive --sim ethereum/eels/consume-engine --client go-ethereum --sim.buildarg tag=glamsterdam-devnet-v8.1.4

# an image built locally with packages/testing/docker/hive/build.sh in execution-specs
./hive --sim ethereum/eels/consume-rlp --client go-ethereum --sim.buildarg image=hive/consume-rlp --sim.buildarg tag=local
```

Pass `--docker.pull` to refresh a moving tag that is already present on the
machine. The tag scheme, how the images are built and how to assemble a
combination that is not published are documented in the execution-specs
repository under `docs/running_tests/hive/images/`.

[docs/commandline.md]: ../../../docs/commandline.md
