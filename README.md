# Viewpoint

End-to-end framework to test `Ethereum` network clients.

## Build

```
$ go build -o viewpoint cmd/main.go
```

## Quickstart

Start the server:

```
$ viewpoint server --name test [--genesis-time 1m] [--num-genesis-validators 10] [--min-genesis-validator-count 10] [--num-tranches 1]
2022-07-15T20:48:31.093+0200 [INFO]  viewpoint: eth1 server deployed: addr=http://127.0.0.1:8001
2022-07-15T20:48:31.093+0200 [INFO]  viewpoint: deposit contract deployed: addr=0xB96198679b67b455FD654aEfC91bF31DC9C0886D
2022-07-15T20:48:35.017+0200 [INFO]  viewpoint: GRPC Server started: addr=localhost:5555
```

The `server` command starts the Viewpoint agent which handles the lifecycle of an Ethereum (post-merge) network. At startup, it generates an initial set of validator accounts (`num-genesis-validators`), splits them in tranches (`num-tranches`) and creates a `genesis.ssz` file. Each validator client will own a single tranche of accounts.

The deployment of the nodes is done using [Docker](https://www.docker.com/) containers which are stopped once the `server` process is over. The agent creates an `e2e-<name>` folder in the root directory where all the metadata, specs and node logs are stored.

Now, lets deploy a validator client for the network.

```
$ viewpoint node deploy validator --type [prysm|lighthouse|teku] --tranche 0 --beacon --beacon-count 2
```

The validator will use the accounts in the tranche `0` (the only one created), which is enough to start the network once the genesis time is reached.

The `node deploy validator` command includes the `--beacon` and `--beacon-count` flags as a convenience method to pre-deploy a set of beacon nodes (with the same consensus client) to which the validator can connect to join the network.

At any point we can deploy another beacon node with:

```
$ viewpoint node deploy beacon --type [prysm|lighthouse|teku]
```

## Commands

### Server

```
$ viewpoint server
```

The `server` command starts the Viewpoint agent.

Flags:

- `name` (`test`): Name of the execution round.
- `num-genesis-validators` (`10`): Number of active validator accounts at genesis.
- `min-genesis-validator-count` (`10`): Number of required active validators to start the chain at genesis.
- `genesis-time` (`1m`): Amount of time from now when the genesis starts.
- `num-tranches` (`1`): Number of tranches. It has to be an exact multiple of `genesis-validator-count`.
- `altair` (`null`): Enable the `Altair` hard fork at a given epoch. Disabled by default.

### Deposit create

```
$ viewpoint deposit create
```

The `deposit create` command creates a new tranche with `num-validators`. For each one, it sends a deposit transaction to the deposit smart contract on the execution node (`Geth`). Eventually, those accounts will be active on the consensus layer.

Flags:

- `num-validators`: Number of accounts for the tranche.

### Deposit list

```
$ viewpoint deposit list
```

The `deposit list` command lists all the tranches.

### Node deploy beacon

```
$ viewpoint deploy beacon
```

The `deploy beacon` command deploys multiple beacon node clients. The nodes use a bootnode to discover each other.

Flags:

- `type`: Client type of the beacon node (`Prysm`, `Lighthouse` or `Teku`).
- `count` (`1`): Number of beacon nodes to deploy.
- `repo`: Override to the default Docker repository for the client.
- `tag`: Override for the default Docker image tag for the client.

### Node deploy validator

```
$ viewpoint deploy validator
```

The `deploy validator` command deploys a validator client.

Flags:

- `type`: Client type of the validator (`Prysm`, `Lighthouse` or `Teku`).
- `num-validators` (`0`): If set, Viewpoint will create a new tranche of `num-validators` accounts (with the deposits).
- `tranche` (`0`): Index of the tranche to use by the validator. It does not take effect if `num-validators` is set.
- `beacon` (`false`): If enabled, pre-deploy a set of beacon nodes to which the validator will connect.
- `beacon-count` (`1`): Number of beacon nodes to deploy if `--beacon` enabled.
- `repo`: Override to the default Docker repository for the client.
- `tag`: Override for the default Docker image tag for the client.

### Node list

```
$ viewpoint node list
```

The `node list` command lists all the running nodes.

### Node status

```
$ viewpoint node status <name>
```

The `node status` command queries the state of a specific node `name`.
