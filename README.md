## golang std lib http/2 throughput speed is much slower than http/1

`go run main.go` on Intel(R) Core(TM) i7-8559U CPU @ 2.70GHz gives:

    server created and listening at :8765 (http1.1)
    server created and listening at :9876 (http/2 cleartext)
    downloading http://localhost:8765/10000000000
    receiving data with HTTP/1.1
    received 10000000000 bytes in 1.996206772s = 40.1 Gbps
    downloading http://localhost:9876/10000000000
    receiving data with HTTP/2.0
    received 10000000000 bytes in 11.259359779s = 7.1 Gbps

so roughly x5 slower
