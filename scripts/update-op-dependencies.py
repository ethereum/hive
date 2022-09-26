#!/usr/bin/env python3
import os
import re
import argparse
import subprocess

MODULES = [
    'optimism',
    'simulators/optimism/l1ops',
    'simulators/optimism/p2p',
    'simulators/optimism/rpc',
    'simulators/optimism/testnet',
]
REPLACER_RE = r'replace github\.com/ethereum/go-ethereum (.*) => github.com/ethereum-optimism/op-geth'
VERSION_RE = r'github\.com/ethereum-optimism/op-geth@([va-f0-9\d\.\-]+)'

parser = argparse.ArgumentParser()
parser.add_argument('--version', help='version to upgrade to')
parser.add_argument('--geth', type=bool, help='update geth rather than op dependencies')


def main():
    args = parser.parse_args()

    if args.version is None:
        raise Exception('Must specify a version.')

    if args.geth:
        update_geth(args)
    else:
        update_op_deps(args)


def update_geth(args):
    for mod in MODULES:
        should_update = False
        with open(os.path.join(mod, 'go.mod')) as f:
            for line in f:
                if re.search(REPLACER_RE, line):
                    original_version = line.strip().split(' ')[2]
                    should_update = True
                    break
        if not should_update:
            continue

        print(f'Updating {mod}')
        run([
            'go',
            'mod',
            'edit',
            '-replace',
            f'github.com/ethereum/go-ethereum@{original_version}=github.com/ethereum-optimism/op-geth@{args.version}'
        ], cwd=mod, check=True)
        tidy(mod)


def update_op_deps(args):
    for mod in MODULES:
        needs = set()
        with open(os.path.join(mod, 'go.mod')) as f:
            for line in f:
                if line.endswith('// indirect\n'):
                    continue

                if not re.search(r'github.com/ethereum-optimism/optimism/op-(\w+)', line):
                    continue

                dep = line.strip().split(' ')[0]
                needs.add(dep)

        print(f'Updating {mod}')
        for need in needs:
            go_get(mod, need, args.version)
        tidy(mod)


def go_get(mod, dep, version, capture_output=False, check=True):
    args = [
        'go',
        'get',
        f'{dep}@{version}'
    ]
    return run(args, cwd=mod, check=check, capture_output=capture_output)


def tidy(mod):
    args = [
        'go',
        'mod',
        'tidy'
    ]
    run(args, cwd=mod, check=True)


def run(args, cwd=None, capture_output=False, check=True):
    print(subprocess.list2cmdline(args))
    return subprocess.run(args, cwd=cwd, check=check, capture_output=capture_output)


if __name__ == '__main__':
    main()
