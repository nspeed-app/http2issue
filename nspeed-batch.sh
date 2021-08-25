#!/bin/sh
curl -o  ./nspeed  https://dl.nspeed.app/nspeed-client/latest/nspeed_linux_amd64
chmod +x ./nspeed
# no encryption
./nspeed -cpu -color server -n 1 get -w 1 http://localhost:7333/20g
./nspeed -cpu -color server -h2c -n 1 get -h2c -w 1 http://localhost:7333/20g
# with encryption
./nspeed -cpu -color server -self -n 1 get -self -http11 -w 1 https://localhost:7333/20g
./nspeed -cpu -color server -self -n 1 get -self -w 1 https://localhost:7333/20g
rm ./nspeed
