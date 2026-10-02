# pbt-artifacts

Conformance for the [EIP-8347](https://eips.ethereum.org/EIPS/eip-8347) offline
migration artifacts, the preimage file and the PBT snapshot. Every client is
handed the same byte-canonical pair and must accept the sound one, reject
every unsound one, and where it can produce an artifact, emit the same
bytes. Clients that lack a feature report it as unsupported, so the run
doubles as a capability matrix.

The PBT snapshot is a stream of tagged records closed by an end tag and the
claimed root ([EIPs#12379](https://github.com/ethereum/EIPs/pull/12379),
[#12403](https://github.com/ethereum/EIPs/pull/12403) and
[#12404](https://github.com/ethereum/EIPs/pull/12404)); the preimage file's
format is unchanged.

The live migration (BAL replay, shadow roots, the fork switch) belongs to
[pbt-devnet](https://github.com/CPerezz/pbt-devnet).

## Running

```bash
./hive --sim ethereum/pbt-artifacts \
       --client-file simulators/ethereum/pbt-artifacts/clients.yaml

# the simulator is its own module, so run the tool from it
go -C simulators/ethereum/pbt-artifacts run ./tools/matrix "$PWD/workspace/logs" > CAPABILITY.md
```

`clients.yaml` pins geth and nethermind to the branches carrying their PBT
work; erigon and besu run stock. To measure a local build, point the stock
client Dockerfile at your image:

```yaml
- client: go-ethereum
  nametag: local
  build_args: {baseimage: my-geth, tag: dev}
```

## Driving a client

One shim per client, uploaded to `/hive-bin/pbt-artifacts.sh` and invoked
over hive's exec channel, with `shims/common.sh` beside it. Nothing under
`clients/` changes; adding a client is one script in `shims/`, named after
the client. A client with no shim gets `shims/unsupported.sh` and shows up as
a gap, not a failure.

| verb | arguments | stdout |
|---|---|---|
| `genesis-root` | | `mpt_root=0x…` |
| `verify` | `<snapshot> <preimages> <anchor>` | `client_exit=<status>` |
| `convert` | `<anchor> [drop-preimage <0xhash>]...` | `snapshot=<b64>` and/or `preimages=<b64>` |

Exit `0` accepted, `1` rejected, `3` unsupported, anything else a crash. A
crash never counts as a rejection, and a shim must not wrap the client in
`|| exit 1`: it echoes the real status as `client_exit=`. Paths are relative
to the fixture tar the simulator uploads; the anchor is block 0.
`convert` prints only what the client can produce: a missing line means
that artifact is unsupported. It prints nothing unless the client exited 0.
Each `drop-preimage` asks for a source whose preimage store lacks that
hash, which converter step 2 must refuse; a client with no such store
answers unsupported.

Where a client has no command line for a verb its shim boots a throwaway
node; see `shims/nethermind.sh`.

Each case records in the generated `manifest.json` the effect its mutation
had on the artifact: byte lengths, the first differing offset, whether the
file still parses, and the leaf keys or record addresses it added, removed,
changed or reordered. The generator computes it from the bytes it wrote, the
generator refuses a set where two cases record the same effect, and every
test carries it in its description. The simulator matches nothing against
a client's error text; a shim may, to tell its client's refusal from a
crash.

## Scoring

- `genesis/state-root` gates everything: a mismatch makes the rest
  `inconclusive`; unsupported is a gap, not a failure, and does the same.
- `<suite>/valid` gates its suite: reject the sound pair and the rest is
  `inconclusive`; unsupported is a gap, not a failure, and the suite becomes
  one `NOT-RUN` row.
- A reject case passes on exit `1` with something on stderr.
- `convert/run` gates production: a crash fails it; unsupported is a gap,
  not a failure.
- `produce/*` cases run only where the sound source converted. Exit `1`
  with something on stderr and no artifact line passes; unsupported is a
  capability, not a failure.
- `agreement/<artifact>` is the run's verdict, not a client's: every
  producer must match the canonical bytes. One diverging fails; none
  matching fails too; a single producer is reported inconclusive.

## Fixtures

Nothing under `fixtures/` is checked in but the generator and `validate.sh`.
The simulator image builds geth at a pinned commit (`geth_commit` in the
Dockerfile) and runs `fixtures/gen`, which writes the anchor genesis, the
valid pair and one file per case.

The genesis has one account per embedding rule. Code sizes around the
31-byte chunk, PUSH32 and PUSH2 straddling a chunk, PUSHDATA running past the
code's end (leading count at the 31 cap), a zero chunk that must be absent and
a trailing one, code entirely zero, code spanning two code groups, shared
bytecode, code that merely starts with `0xef0100` at 23 and 24 bytes,
delegations to a shared target, a distinct one, a missing account, an EOA and
another delegation, storage either side of the header split and across
groups, an account with only overflow storage, a contract with zero balance
and nonce, maximum nonce and balance. The 24 KiB maximum is left out: 793
leaves for nothing the two-group case does not cover. The produce leg is
measured on this sound state only.

The manifest names every case and its clause. A `snapshot.*` or
`preimages.*` clause is a rule on the bytes; any other clause is one the
dual-check enforces. The generator holds every case to its clause: a
`snapshot.*`/`preimages.*` case must fail the generator's strict decoder
with exactly that rule, and every other case must pass it. Every case is
scored, including the empty snapshot, which the dual-check accepts only
against the empty MPT root; the fixture anchor holds accounts.

Some cases carry the valid leaf set in a second encoding: a stem or an
account's storage split over two records, a one-leaf group in the multi-leaf
form, a record out of its zone. The tree and the dual-check agree with the
valid file on these, so only the byte rules reject them, and those are what
keep independent producers' digests comparable.

The preimage file is derived from the allocation, not taken from a client.
The snapshot needs a tree, so its bytes come from the reference converter,
held to four checks before any case is cut from it: its values, chunking and
presence against a derivation of the embedding rules that shares only geth's
key functions; its serialization against the generator's own strict decoder;
the generator's encoder reproducing it byte for byte, since every case is
written by that encoder; and both files' digests against the pair admitted
when the fixtures were designed, whose root execution-specs' reference state
model computes too. Keys and root otherwise rest on geth's `trie/bintrie`,
which is why the pair is pinned and why `agreement/snapshot` is what makes
the bytes a cross-client claim. A converter that writes different bytes
fails the image build.

To look at the fixtures or to admit a changed pair, run the generator
locally. It needs the go-ethereum PBT fork checked out beside `hive/`, and
for the root gate an execution-specs checkout with its `.venv`:

```bash
cd fixtures/gen && cp go.mod.dist go.mod && cp go.sum.dist go.sum
go run -tags pbtgen . -geth /path/to/geth -ref /path/to/execution-specs -out /tmp/fixtures
cd .. && ./validate.sh /path/to/geth /path/to/execution-specs   # the same judgement, no docker
```
