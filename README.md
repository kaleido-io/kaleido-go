# kaleido-go

Chain exerciser example in Golang for Ethereum permissioned chains.

Built as a sample and exerciser for the [kaleido.io](https://kaleido.io) platform,
and compatible with other Ethereum chains.

The exerciser uses the go-ethererum JSON/RPC client to provide a simple command-line
interface and code samples for:
- Compiling and executing Solidity smart contracts
- Submitting transactions to be signed by the go-ethereum (geth) node
- Externally signing transactions (using [EIP155](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-155.md) signing)
- Checking for transaction receipts

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
  -a, --accounts stringArray   Account addresses - 1 per worker needed for internal geth client signing
  -x, --args stringArray       String arguments to pass to contract method
  -i, --chainid int            Chain ID - required for external signing
  -c, --contract string        Pre-deployed contract address. Will be deployed if not specified
  -d, --debug int              0=error, 1=info, 2=debug (default 1)
  -e, --extsign                Sign externally with generated accounts
  -f, --file string            Solidity smart contract to compile (and deploy if no contract address supplied)
  -g, --gas int                Gas limit on the transaction (default 1000000)
  -G, --gasprice int           Gas price
  -h, --help                   help for kaleido-go
  -l, --loops int              How many times each looper should loop till exiting (0=infinite) (default 1)
  -m, --method string          Method in the contract to invoke
  -S, --seconds-max int        Time in seconds before timing out waiting for a txn receipt (default 20)
  -s, --seconds-min int        Time in seconds to wait before checking for a txn receipt (default 11)
  -t, --transactions int       Count of transactions submit on each worker loop (default 1)
  -u, --url string             JSON/RPC URL for the Ethereum node: https://user:pass@xyz-rpc.kaleido.io
  -w, --workers int            Number of workers to run (default 1)
```

