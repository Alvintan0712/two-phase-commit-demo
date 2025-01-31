# Two-phase Commit in Go

This is a demo two-phase commit (2PC) in Go example.

Imitate user place an order and deduct the wallet balance in user database and order database.

# Getting Started

## Init Project

Generate the protobuf API code

```sh
cd shared
make
```

## Start the project
```sh
docker compose up --build -d
```

```sh
curl -X POST http://localhost:8000/order
```

# References
[Alibaba Cloud Blog](https://www.alibabacloud.com/blog/tech-insights---two-phase-commit-protocol-for-distributed-transactions_597326)

[ZooKeeper](https://zookeeper.apache.org/doc/current/recipes.html#sc_recipes_twoPhasedCommit)
