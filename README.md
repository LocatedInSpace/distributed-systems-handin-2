# How to run
Inside of `pseudo_server.go` & `pseudo_client.go`, there are two example usages of my TCP-model - it is important that they match, so that if `pseudo_server.go` has been switched over to the pinging example, that `pseudo_client.go` also has.

 1. Comment out the example you wish to run in each file.

 2. Run each of the files in separate terminals - the order of `forwarder.go` is important, it needs to be run first, the same is not true for `pseudo_client.go` and `pseudo_server.go`:

    ```console
    $ go run forwarder.go
    $ go run pseudo_server.go
    $ go run pseudo_client.go
    ```

 3. Run the server:

    ```console
    $ $(go env GOPATH)/bin/greeter_server &
    ```

 4. Run the client:

    ```console
    $ $(go env GOPATH)/bin/greeter_client
    Greeting: Hello world
    ```

For more details (including instructions for making a small change to the
example code) or if you're having trouble running this example, see [Quick
Start][].

[quick start]: https://grpc.io/docs/languages/go/quickstart
