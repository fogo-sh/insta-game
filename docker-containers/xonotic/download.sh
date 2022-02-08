#!/usr/bin/env sh

FILE=xonotic.zip
SHA_256_SUM=a22f7230f486c5825b55cfdadd73399c9b0fae98c9e081dd8ac76eca08359ad5

validate() {
  echo "$SHA_256_SUM $FILE" | sha256sum -c
}

exists() {
  echo "$FILE exists, validating checksum"
  validate
}

if [ -f "$FILE" ]; then
  exists
else 
  echo "$FILE does not exist, downloading..."
  curl https://dl.xonotic.org/xonotic-0.8.2.zip -L -o xonotic.zip
  exists
fi
