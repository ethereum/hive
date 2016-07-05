# hive - Ethereum end-to-end test harness

Ethereum grew large to the point where testing implementations is a huge burden. Unit tests are fine
for checking various implementation quirks, but validating that a client conforms to some baseline
quality or validating that clients can play nicely together in a multi client environment is all but
simple.

This project is meant to serve as an easily expandable test harness where **anyone** can add tests in
**any** programming language felt comfortable with and it should simultaneously be able to run those
tests against all potential clients. As such the harness is meant to do black box testing where no
client specific internal details/state can be tested and/or inspected, rather emphasis being put on
adherence to official specs or behaviours under different circumstances.

Most importantly, it is essential to be able to run this test suite as part of the CI workflow! To
this effect the entire suite is based on docker containers.

# Contributions

This project takes a different approach to code contributions than your usual FOSS project with well
ingrained maintainers and relatively few external contributors. It is an experiment, whether it will
work out or not is for the future to decide.

We follow the [Collective Code Construction Contract (C4)](http://rfc.zeromq.org/spec:22/C4/), code
contribution model, as expanded and explained in [The ZeroMQ Process](https://hintjens.gitbooks.io/social-architecture/content/chapter4.html).
The core idea being that any patch that successfully solves an issue (bug/feature) and doesn't break
any existing code/contracts **must** be optimistically merged by maintainers. Followup patches may
be used to for additional polishes – and patches may even be outright reverted if they turn out to
have a negative impact – but no change must be rejected based on personal values.

Please consult the two C4 documents for details:
 * [Collective Code Construction Contract (C4)](http://rfc.zeromq.org/spec:22/C4/)
 * [The ZeroMQ Process](https://hintjens.gitbooks.io/social-architecture/content/chapter4.html)

## Ethereum clients

## Client validators

# License

The `hive` project is licensed under the [GNU General Public License v3.0](http://www.gnu.org/licenses/gpl-3.0.en.html),
also included in our repository in the COPYING file.
