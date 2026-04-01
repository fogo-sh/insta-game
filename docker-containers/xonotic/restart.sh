#!/usr/bin/env sh

TOKEN="${TOKEN:-abc123}"

curl --header "Authorization: Bearer ${TOKEN}" localhost:5001/restart
