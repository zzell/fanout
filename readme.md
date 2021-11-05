# fanout
## Run 
Start nats and redis:
```
docker-compose up -d nats redis
```
Dependencies:
```
go mod download
```

Run the app:
```
go build -o fanout . && ./fanout
```

## Test
Run integration tests (make sure nats and redis are running):
```
make integration
```

Run unit tests:
```
make test
```

## Code quality
Run linter (requires golangci-lint):
```
make lint
```
