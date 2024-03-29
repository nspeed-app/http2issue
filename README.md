[![SLSA go releaser](https://github.com/nspeed-app/http2issue/actions/workflows/go-ossf-slsa3-publish.yml/badge.svg)](https://github.com/nspeed-app/http2issue/actions/workflows/go-ossf-slsa3-publish.yml)

## this is a PoC to test HTTP1.1 vs HTTP/2 (H2C-no encryption) with golang

a binary is released
## golang std lib http/2 throughput speed is much slower than http/1

`go run main.go` on Intel(R) Core(TM) i7-8559U CPU @ 2.70GHz gives:

    server created and listening at 8765 (http1.1)
    server created and listening at 9876 (http/2 cleartext)
    downloading http://localhost:8765/10000000000
    receiving data with HTTP/1.1
    server sent 10000000000 bytes in 1.448894821s = 55.2 Gbps (262144 chunks)
    client received 10000000000 bytes in 1.448774771s = 55.2 Gbps, 189969 write ops, 2360568 buff 
    downloading http://localhost:9876/10000000000
    receiving data with HTTP/2.0
    server sent 10000000000 bytes in 8.313647415s = 9.6 Gbps (262144 chunks)
    client received 10000000000 bytes in 8.313532464s = 9.6 Gbps, 429444 write ops, 966656 buff 


so roughly x5 slower

with curl: launch `go run main.go -s` and in a separate shell:

    curl -o /dev/null  http://localhost:8765/10000000000
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
    100 9536M  100 9536M    0     0  5397M      0  0:00:01  0:00:01 --:--:-- 5394M    

    curl -o /dev/null --http2-prior-knowledge http://localhost:9876/10000000000
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                    Dload  Upload   Total   Spent    Left  Speed
    100 9536M  100 9536M    0     0   958M      0  0:00:09  0:00:09 --:--:--  974M

With nspeed: download nspeed at http://nspeed.app/ 
or execute [nspeed-batch.sh](nspeed-batch.sh)

http/1.1 vs http/2 no encryption:

    # http/1.1
    ./nspeed_linux_amd64 server -n 1 get -w 1 http://localhost:7333/10g
    # http/2 clear text
    ./nspeed_linux_amd64 server -n 1 get -h2c -w 1 http://localhost:7333/10g

http/1.1 vs http/2 with encryption:

    # http/1.1
    ./nspeed_linux_amd64 server -self -n 1 get -self -http11 -w 1 https://localhost:7333/10g
    # http/2
    ./nspeed_linux_amd64 server -self -n 1 get -self -w 1 https://localhost:7333/10g

example results: see [nspeed.results.txt](nspeed.results.txt)

### Caddy

    #https/2
    curl -o /dev/null https://localhost:8082/10G.iso
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                    Dload  Upload   Total   Spent    Left  Speed
    100 9536M  100 9536M    0     0   484M      0  0:00:19  0:00:19 --:--:--  483M

    #https/1.1
    curl -o /dev/null --http1.1 https://localhost:8082/10G.iso
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                    Dload  Upload   Total   Spent    Left  Speed
    100 9536M  100 9536M    0     0   929M      0  0:00:10  0:00:10 --:--:--  935M

    #http/1.1 (no encryption - max throughput reference)
    curl -o /dev/null http://localhost:8081/10G.iso
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                    Dload  Upload   Total   Spent    Left  Speed
    100 9536M  100 9536M    0     0  1672M      0  0:00:05  0:00:05 --:--:-- 1687M
