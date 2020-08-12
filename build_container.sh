#!/bin/bash

docker build . -t propellerfactory/gifski:latest
IMAGEID=`docker create propellerfactory/gifski:latest`
docker cp $IMAGEID:/usr/src/app/target/release/libgifski.a .
docker rm $IMAGEID