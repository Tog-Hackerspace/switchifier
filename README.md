Tog Switchifier Service
=======================

This is a small service that collects status updates from the knife switch at Tog.

Building
--------

I assume you already have a GOPATH set up. This code was tested with and developed againat Go 1.7.

    $ go get github.com/Tog-Hackerspace/switchifier
    $ go build github.com/Tog-Hackerspace/switchifier
    $ file switchifier

Runnning
--------

This binary binds to a TCP port and listens for incoming TCP requests. It uses a SQLite database to store historical state data, and keeps a few other things (like last ping from client) in memory.

Since this service authenticates its' client by a preshared secret, you should probably reverse-proxy this and wrap it in TLS or use another secure tunnel.

Here's a few useful flags:

 - `--db_path`: Path to the SQLite database. The default is `switchifier.db` in the current working directory.
 - `--bind_address`: Host/port to bind to. By default `:8080`, so port 8080 on all interfaces.
 - `--secret`: The preshared secret used to authenticate the client.


Future work
-----------

 - SSL client cert authentication support.
 - GRPC API.
 - State overrides.

License
-------

See COPYING.

API
===

status
------
`GET /api/1/switchifier/status` - Get current switch status.

Takes no parameters.

Returns a JSON dictrionary with the following keys:

 - `okay` - true if request was successful, false if an error occured.
 - `error` - a string detailing the error that occured if `okay` is false, else empty.
 - `data` - a dictionary containing response data if `okay` is true, else undefined.
    - `open` - true if the space is currently marked as open, else false
    - `since` - nanoseconds since Unix epoch of the last `data.open` state change
    - `lastKeepalive` - nanoseconds since Unix epoch of the last client update. May be zero if data is unavailable.

Example:

    $ curl 127.0.0.1:8080/api/1/switchifier/status
    {"okay":true,"error":"","data":{"open":true,"since":1475744318547307236,"lastKeepalive":1475744318562059736}}
 
update
------
`POST /api/1/switchifier/update` - Update knife switch status from client.

Takes the following form parameters:

 - `secret` - the preshared secret required for client updates.
 - `status` - the state of the current status. Truthy (open) values are `[Tt]rue`, `1`. Falsey (closed) values are everything else.

Returns 200 if the update was succesfull. All other 5xx and 4xx codes shall be interpreted as errors. The client should periodically call this endpoint regardless of success.

Example:

    #!/usr/bin/env python3
    import requests

    r = requests.post('http://127.0.0.1:8080/api/1/switchifier/update', data={
        'secret': "changeme",
        'value': "true",
    })

