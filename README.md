# kaleido-go

Chain exerciser example in Golang for Ethereum permissioned chains.

Built as a sample and exerciser for the [kaleido.io](https://kaleido.io) platform,
and compatible with other Ethereum chains.

Tested with [go-ethereum](https://github.com/ethereum/go-ethereum/) and
[Quorum](https://github.com/jpmorganchase/quorum) and [Hyperledger Besu](https://www.hyperledger.org/projects/besu) permissioned chains.

The exerciser uses the go-ethererum JSON/RPC client to provide a simple command-line
interface and code samples for:
- Compiling and executing Solidity smart contracts
- Submitting transactions to be signed by the go-ethereum (geth) node
- Externally signing transactions (using [EIP155](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-155.md) signing)
- Checking for transaction receipts
- Private Transactions compatible with Quorum, and the [EEA Enterprise Ethereum Client Specification V1](https://entethalliance.org/resources/)

## Installing (Mac/Linux/Windows)

Grab the latest binary release from: https://github.com/kaleido-io/kaleido-go/releases
Download, unzip and run.

## License

This code is distributed under the [Apache 2 license](LICENSE).

> Please note the code currently statically links to code distributed under the
> LGPL license.

## Running

```
Sample exerciser for Ethereum permissioned chains - from Kaleido

Usage:
  kaleido-go [flags]

Flags:
  -a, --accounts stringArray       Account addresses - 1 per worker needed for geth signing
  -x, --args stringArray           String arguments to pass to contract method (auto-converted to type)
  -C, --call                       Call the contract and return a value, rather than sending a txn
  -i, --chainid int                Chain ID for EIP155 signing (networkid queried if omitted)
  -c, --contract string            Pre-deployed contract address. Will be deployed if not specified
  -n, --contractname string        The name of the contract to call, for Solidity files with multiple contracts
  -d, --debug int                  0=error, 1=info, 2=debug (default 1)
  -E, --estimategas                Estimate the gas for the contract call, rather than sending a txn
  -e, --extsign                    Sign externally with generated private keys + accounts
  -f, --file string                Solidity smart contract source. Deployed if --contract not supplied
  -F, --flush-period int           Flush period for statsd metrics (ms) (default 1000)
  -g, --gas int                    Gas limit on the transaction (default 1000000)
  -G, --gasprice int               Gas price
  -h, --help                       help for kaleido-go
  -k, --keys string                JSON file to create/update with an array of private keys for extsign
  -l, --loops int                  Loops to perform in each worker before exiting (0=infinite) (default 1)
  -m, --method string              Method name in the contract to invoke
  -M, --metrics string             statsd server to submit metrics to
  -q, --metrics-qualifier string   Additional metrics qualifier
  -N, --nonce int                  Nonce (transaction number) for the next transaction (default -1)
  -P, --privateFor stringArray     Private for (see EEA Client Spec V1)
  -p, --privateFrom string         Private from (see EEA Client Spec V1)
  -R, --rpc-timeout int            Timeout in seconds for an individual RCP call (default 30)
  -S, --seconds-max int            Time in seconds before timing out waiting for a txn receipt (default 20)
  -s, --seconds-min int            Time in seconds to wait before checking for a txn receipt (default 11)
  -T, --telegraf                   Telegraf/InfluxDB stats naming (default is Graphite)
  -t, --transactions int           Count of transactions submit on each worker loop (default 1)
  -u, --url string                 JSON/RPC URL for Ethereum node: https://user:pass@xyz-rpc.kaleido.io
  -w, --workers int                Number of workers to run (default 1)
```

## Build

```sh
make
```

## Examples

Example commands below expect you to have set the following parameters:

```sh
# The full Node URL including any application credentials
NODE_URL="https://user:pass@nodeurl-rpc.kaleido.io"
# Account existing on the node
ACCOUNT=0x0102030405060708090a0b0c0e0e0f1011121314
```

# Deploy a contract and call a simple transaction

Shell Command (linux/mac):

```sh
./kaleido-go -f examples/simplestorage.sol \
  -m set -x 12345 \
  -u "$NODE_URL" -a "$ACCOUNT"
```

> You can enable DEBUG output with `-d 2`

Example output:

```
INFO[2018-05-14T22:58:41-04:00] Exercising method 'set' in solidity contract examples/simplestorage.sol
INFO[2018-05-14T22:58:42-04:00] Deploying contract using worker W0000
INFO[2018-05-14T22:58:42-04:00] W0000/L0000/N000127: TX:0x739746d5b0c8bfb85e9975bd6ba5b5de52debd4e5119b556e22370ebc6947bdf Sent. OK=true [0.06s]
INFO[2018-05-14T22:58:54-04:00] W0000/L0000/N000128: TX:0x739746d5b0c8bfb85e9975bd6ba5b5de52debd4e5119b556e22370ebc6947bdf Mined=true after 11.18s [0.18s]
INFO[2018-05-14T22:58:54-04:00] Contract address=0x2C13d6D15975EfbF7DfD2bFdaFe7413e391eFc65
INFO[2018-05-14T22:58:54-04:00] W0000/L0000/N000128: TX:0x51a62f3922f59405e86f85d79ad79bfcfdf24305ac4b103c6fe0ca6cd97105c3 Sent. OK=true [0.04s]
INFO[2018-05-14T22:59:05-04:00] W0000/L0000/N000129: TX:0x51a62f3922f59405e86f85d79ad79bfcfdf24305ac4b103c6fe0ca6cd97105c3 Mined=true after 11.17s [0.17s]
```

# Call the deployed contract to get the value


Shell Command (linux/mac):

```sh
./kaleido-go -f examples/simplestorage.sol \
  -m get -C -c 0x2C13d6D15975EfbF7DfD2bFdaFe7413e391eFc65 \
  -u "$NODE_URL" -a "$ACCOUNT"
```

> TODO: Perform data-type sensitive parsing of return values

Example output:
```
INFO[2018-05-14T23:01:26-04:00] Exercising method 'get' in solidity contract examples/simplestorage.sol
INFO[2018-05-14T23:01:26-04:00] Contract address=0x2C13d6D15975EfbF7DfD2bFdaFe7413e391eFc65
INFO[2018-05-14T23:01:26-04:00] W0000/L0000/N000129: Call result: '0x0000000000000000000000000000000000000000000000000000000000003039' [0.04s]
```

# Call the deployed contract to get the value in loop

Run 10 contract calls with 1 second delay between each call

Shell command (linux/mac):

```sh
./kaleido-go -f examples/simplestorage.sol \
  -m get -C -c 0x2C13d6D15975EfbF7DfD2bFdaFe7413e391eFc65 \
  -u "$NODE_URL" -a "$ACCOUNT"
  -l 10 -s 1
```


# Send 10 private transactions with debug and custom wait times

> Private transactions can only currently be signed on the node

Shell Command (linux/mac):

```sh
./kaleido-go -f examples/simplestorage.sol -m set -x 12345 \
  -t 10 \
  -p xoA1FQSUHpslp7lJWgRQXDErm/nsP7/CpBOWf6M3VTI= -P p3+cTaDZ9Lw1EPlJlcM9hhezlXTqAEi6xi+LTDIdW2E= \
  -s 15 -S 30 -d 3 \
  -u "$NODE_URL" -a "$ACCOUNT"
```

# Send an externally signed transaction

> The private keys will be randomly generated (and discarded), so this is only an example of external signing logic for exercising a node

Shell Command (linux/mac):

```sh
./kaleido-go -f examples/simplestorage.sol -m set -x 12345 -u "$NODE_URL" -e
```
