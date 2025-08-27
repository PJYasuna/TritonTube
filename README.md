# TritonTube

## Description
A youtube-like website built in Go, supporting uploads, MPEG-DASH streaming, and SQLite-based metadata storage.
Scaled storage using consistent hashing and gRPC for horizontal scalability and dynamic node reconfiguration.
Designed custom protocols for video transfer and implemented automated file migration during cluster reconfiguration.

## A example to run
You can start up the service by running the following command in the project root.

```
mkdir -p storage/8090 storage/8091 storage/8092

go run ./cmd/storage -port 8090 "./storage/8090" # storage 8090
go run ./cmd/storage -port 8091 "./storage/8091" # storage 8091
go run ./cmd/storage -port 8092 "./storage/8092" # storage 8092

go run ./cmd/web \
    sqlite "./metadata.db" \
    nw     "localhost:8081,localhost:8090,localhost:8091,localhost:8092"
```

It starts three storage servers (8090, 8091, 8092) and the web server. We use SQLite to store metadata in metadata.db and use the new nw content service. Its option is a comma-separated list of addresses. The first address is the host and port of the admin server. In this case, the web server will start the gRPC server for VideoContentAdminService defined in admin.proto at localhost:8081. The rest of the list is addresses of storage servers. Since we’re running all nodes on the same machine, we specify localhost with the three ports.

Use the browser to make sure the website works as expected. You can use the same testing strategy from Lab 7.

Then, use the admin CLI to modify the cluster.

```
$ go run ./cmd/admin list localhost:8081
$ go run ./cmd/admin remove localhost:8081 localhost:8090
$ go run ./cmd/admin add localhost:8081 localhost:8090
```


