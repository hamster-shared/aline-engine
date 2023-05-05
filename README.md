<div align="center">
    <img src="https://socialify.git.ci/hamster-shared/aline-engine/image?description=1&descriptionEditable=One-stop%20Toolkit%20and%20Middleware%20Platform%20for%20Web3.0%20Developers&font=KoHo&logo=https%3A%2F%2Fhamsternet.io%2F_nuxt%2Flogo.668de5a2.png&owner=1&pattern=Floating%20Cogs&theme=Auto" width="640" height="320" alt="logo" />

# Aline Engine

[![Discord](https://badgen.net/badge/icon/discord?icon=discord&label)](https://discord.gg/qMWUvs7jkV)
[![Telegram](https://badgen.net/badge/icon/telegram?icon=telegram&label)](https://t.me/hamsternetio)
[![Twitter](https://badgen.net/badge/icon/twitter?icon=twitter&label)](https://twitter.com/Hamsternetio)

</div>

This project is a workflow engine similar to GitHub Actions, designed for automating the deployment, testing, verification, monitoring, and other functionalities of on-chain contracts.

The engine is divided into worker nodes and master nodes, using gRPC for communication.

## Worker Nodes

```go
func NewWorkerEngine(masterAddress string) (Engine, error) {}
```

## Master Nodes

```go
func NewMasterEngine(listenPort int) (Engine, error) {}
```

## Usage Example

This project primarily serves the [hamster-develop](https://github.com/hamster-shared/hamster-develop) project. You can refer to the usage in that project for more information.

## Documentation

[Documentation](https://pkg.go.dev/github.com/hamster-shared/aline-engine)

## About Hamster

Hamster is aiming to build the one-stop infrastructure developer toolkits for Web3.0. It defines itself as a development, operation and maintenance DevOps service platform, providing a set of development tools as well as O&M tools, empowering projects in Web3.0 to improve their coding and delivery speed, quality and efficiency, as well as product reliability & safety.

With Hamster, developers or project teams realize the development, verification and O&M stages of their blockchain projects in an automatic, standardized and tooled approach: from contract template of multiple chains, contract/frontend code build, security check, contract deployment to the contract operation and maintenance.

Together with its developer toolkits, Hamster offers the RPC service and decentralized computing power network service when users choose to deploy their contracts via Hamster.

At the same time, the contract security check part within the developer toolkits is offered separately to its to-C customers, who could check their contracts to avoid potential security risks.

## Contributors

This project exists thanks to all the people who contribute.

 <a href="https://github.com/hamster-shared/aline-engine/contributors">
  <img src="https://contrib.rocks/image?repo=hamster-shared/aline-engine" />
 </a>

## License

[MIT](LICENSE)
