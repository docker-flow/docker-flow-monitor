#!/usr/bin/env bash

docker image build -t vfarcic/alert-manager .

docker push vfarcic/alert-manager

docker image tag vfarcic/alert-manager vfarcic/alert-manager:demo

docker push vfarcic/alert-manager:demo